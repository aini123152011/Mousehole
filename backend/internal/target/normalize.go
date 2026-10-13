package target

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"mousehole/backend/internal/validation"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// AlgorithmVersion 固定当前规范化解释；依赖升级不能重写历史内容。
const AlgorithmVersion = "target-v1"

var domainProfile = idna.New(idna.MapForLookup(), idna.Transitional(false), idna.StrictDomainName(true), idna.ValidateLabels(true), idna.VerifyDNSLength(true), idna.BidiRule())

// Normalize 规范化明确类型的输入，不能将 URL 静默降格为域名。
func Normalize(kind, value string) (string, error) {
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", validation.New("", "原始输入不能包含控制字符")
	}
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || value == "" || len(value) > 2048 || strings.IndexFunc(value, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 {
		return "", validation.New("", "输入为空、过长或含空白及控制字符")
	}
	switch kind {
	case "domain":
		return Domain(value)
	case "ip":
		return IP(value)
	case "cidr":
		return CIDR(value)
	case "url", "url_exact":
		return URL(value)
	default:
		return "", validation.New("", "不支持的目标类型")
	}
}

// Domain 使用严格非 transitional IDNA；匹配不会自动提升到根域。
func Domain(value string) (string, error) {
	if strings.ContainsAny(value, ":/\\@%?#*") {
		return "", validation.New("", "域名不能包含 URL、端口或通配符")
	}
	value = strings.TrimSuffix(value, ".")
	ascii, err := domainProfile.ToASCII(value)
	if err != nil || ascii == "" || len(ascii) > 253 {
		return "", validation.New("", "域名 IDNA 或长度无效")
	}
	ascii = strings.ToLower(ascii)
	for _, label := range strings.Split(ascii, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", validation.New("", "域名标签无效")
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return "", validation.New("", "域名标签包含非法字符")
			}
		}
	}
	if _, err := netip.ParseAddr(ascii); err == nil {
		return "", validation.New("", "IP 字面量必须使用 IP 类型")
	}
	return ascii, nil
}

// IP 以二进制地址族为准，mapped IPv6 与 IPv4 使用相同自然键。
func IP(value string) (string, error) {
	address, err := netip.ParseAddr(value)
	if err != nil || address.Zone() != "" {
		return "", validation.New("", "IP 必须为标准地址且不能带 zone")
	}
	return address.Unmap().String(), nil
}

// CIDR 拒绝主机位和 mapped prefix，避免隐式扩大用户授权范围。
func CIDR(value string) (string, error) {
	prefix, err := netip.ParsePrefix(value)
	if err != nil || prefix.Addr().Zone() != "" || prefix.Addr().Is4In6() {
		return "", validation.New("", "CIDR 无效或使用了不支持的映射前缀")
	}
	if prefix != prefix.Masked() {
		return "", validation.New("", fmt.Sprintf("CIDR 含主机位，请确认网络范围后使用 %s", prefix.Masked()))
	}
	return prefix.String(), nil
}

// URL 保留路径及 query 的语义差异；仅去默认端口和统一路径转义 hex。
func URL(value string) (string, error) {
	if strings.ContainsAny(value, "\\#") {
		return "", validation.New("", "URL 不接受反斜杠或片段")
	}
	u, err := url.Parse(value)
	if err != nil || u.Opaque != "" || u.User != nil || u.Host == "" || (strings.ToLower(u.Scheme) != "http" && strings.ToLower(u.Scheme) != "https") {
		return "", validation.New("", "必须为无凭据的绝对 HTTP(S) URL")
	}
	host := u.Hostname()
	var normalized string
	if _, parseErr := netip.ParseAddr(host); parseErr == nil {
		normalized, err = IP(host)
	} else {
		normalized, err = Domain(host)
	}
	if err != nil {
		return "", err
	}
	u.Scheme = strings.ToLower(u.Scheme)
	port := u.Port()
	if strings.HasSuffix(u.Host, ":") {
		return "", validation.New("", "URL 端口不能为空")
	}
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
			return "", validation.New("", "URL 端口无效")
		}
		if u.Scheme == "http" && n == 80 || u.Scheme == "https" && n == 443 {
			port = ""
		}
	}
	if port != "" {
		u.Host = net.JoinHostPort(normalized, port)
	} else if strings.Contains(normalized, ":") {
		u.Host = "[" + normalized + "]"
	} else {
		u.Host = normalized
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	lowerPath := strings.ToLower(path)
	if strings.Contains(lowerPath, "%2f") || strings.Contains(lowerPath, "%5c") {
		return "", validation.New("", "URL 路径不能包含编码的分隔符")
	}
	for _, segment := range strings.Split(path, "/") {
		decoded, err := url.PathUnescape(segment)
		if err != nil || decoded == "." || decoded == ".." || strings.IndexFunc(decoded, unicode.IsControl) >= 0 {
			return "", validation.New("", "URL 路径包含点段或控制字符")
		}
	}
	if _, err := url.QueryUnescape(u.RawQuery); err != nil {
		return "", validation.New("", "URL 查询转义无效")
	}
	// 不使用 path.Clean/Values.Encode，避免扩大端点或改变重复 query 顺序。
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		b.WriteByte(path[i])
		if path[i] == '%' {
			b.WriteString(strings.ToUpper(path[i+1 : i+3]))
			i += 2
		}
	}
	suffix := ""
	if u.RawQuery != "" || u.ForceQuery {
		suffix = "?" + u.RawQuery
	}
	return u.Scheme + "://" + u.Host + b.String() + suffix, nil
}

// SubdomainsAllowed 拒绝 ICANN 与 PRIVATE 裸后缀；企业内部名称仍可显式授权。
func SubdomainsAllowed(domain string) bool {
	suffix, icann := publicsuffix.PublicSuffix(domain)
	return suffix != domain || !icann && !strings.Contains(domain, ".")
}

// DomainMatches 按标签边界匹配，不接受子串或近似名称。
func DomainMatches(host, domain string, include bool) bool {
	return host == domain || include && strings.HasSuffix(host, "."+domain)
}

// Matches 对规范目标进行类型匹配；直接 IP 不借域名获得权限。
func Matches(kind, rule string, include bool, targetKind, value string) bool {
	switch kind {
	case "domain":
		return targetKind == "domain" && DomainMatches(value, rule, include)
	case "ip":
		return targetKind == "ip" && value == rule
	case "cidr":
		p, e := netip.ParsePrefix(rule)
		a, ae := netip.ParseAddr(value)
		return targetKind == "ip" && e == nil && ae == nil && p.Contains(a.Unmap())
	case "url_exact":
		return targetKind == "url" && value == rule
	}
	return false
}

// URLHost 返回规范 URL 的主机类型、值与实际端口，仅作本地拆解。
func URLHost(value string) (string, string, int) {
	u, _ := url.Parse(value)
	kind := "domain"
	if _, err := netip.ParseAddr(u.Hostname()); err == nil {
		kind = "ip"
	}
	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if u.Port() != "" {
		port, _ = strconv.Atoi(u.Port())
	}
	return kind, u.Hostname(), port
}

var reservedPrefixes = []string{"0.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16", "224.0.0.0/4", "240.0.0.0/4", "100.100.100.200/32", "::/128", "::1/128", "fe80::/10", "ff00::/8", "fd00:ec2::254/128"}

// Reserved 拒绝平台内置地址；私网需要明确范围，不能一概当作保留地址。
func Reserved(kind, value string) bool {
	if kind == "url" {
		kind, value, _ = URLHost(value)
	}
	if kind == "domain" {
		return value == "localhost" || strings.HasSuffix(value, ".localhost") || value == "metadata.google.internal"
	}
	if kind != "ip" {
		return false
	}
	a, err := netip.ParseAddr(value)
	if err != nil {
		return true
	}
	for _, entry := range reservedPrefixes {
		if netip.MustParsePrefix(entry).Contains(a.Unmap()) {
			return true
		}
	}
	return false
}
