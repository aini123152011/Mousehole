package target_test

import (
	"strings"
	"testing"

	"mousehole/backend/internal/target"
)

// 这些合同用例只调用纯函数，不注入 resolver、HTTP client 或目标服务。
// .invalid 名称和文档地址只作为字符串，测试本身不解析或连接它们。
func TestNormalizeDomainContract(t *testing.T) {
	t.Parallel()
	longLabel := strings.Repeat("a", 63)
	maxDomain := longLabel + "." + longLabel + "." + longLabel + "." + strings.Repeat("a", 61)
	tests := []struct {
		name, input, want string
		invalid           bool
	}{
		{name: "大小写与合法尾点", input: "  EXAMPLE.COM.  ", want: "example.com"},
		{name: "中文IDNA", input: "例子.测试", want: "xn--fsqu00a.xn--0zwm56d"},
		{name: "非transitional不合并德文字符", input: "faß.de", want: "xn--fa-hia.de"},
		{name: "IDNA点映射", input: "例子。测试", want: "xn--fsqu00a.xn--0zwm56d"},
		{name: "合法企业内部名称", input: "INTRANET", want: "intranet"},
		{name: "内部域名不提升根域", input: "a.corp.internal", want: "a.corp.internal"},
		{name: "最大标签和总长度", input: maxDomain, want: maxDomain},
		{name: "空值", input: " ", invalid: true},
		{name: "原始换行不能先trim隐藏", input: "\nexample.com", invalid: true},
		{name: "原始制表符不能先trim隐藏", input: "example.com\t", invalid: true},
		{name: "原始Unicode控制字符", input: "\u0085example.com", invalid: true},
		{name: "中间空白", input: "exam ple.com", invalid: true},
		{name: "重复尾点", input: "example.com..", invalid: true},
		{name: "空标签", input: "a..example.com", invalid: true},
		{name: "前导连字符", input: "-a.example.com", invalid: true},
		{name: "尾部连字符", input: "a-.example.com", invalid: true},
		{name: "下划线", input: "a_b.example.com", invalid: true},
		{name: "裸通配符", input: "*.example.com", invalid: true},
		{name: "端口", input: "example.com:443", invalid: true},
		{name: "域名入口不能降格URL", input: "https://example.com/path", invalid: true},
		{name: "IP字面量类型不同", input: "192.0.2.1", invalid: true},
		{name: "无效punycode", input: "xn--.example.com", invalid: true},
		{name: "无上下文joiner", input: "a\u200db.example.com", invalid: true},
		{name: "无效UTF8", input: "a\xff.example.com", invalid: true},
		{name: "标签超过63字节", input: strings.Repeat("a", 64) + ".example.com", invalid: true},
		{name: "总长度超过253字节", input: maxDomain + "a", invalid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := target.Normalize("domain", test.input)
			if test.invalid {
				if err == nil || got != "" {
					t.Fatalf("无效域名应拒绝且无规范值，得到 %q / %v", got, err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("规范域名 = %q / %v，预期 %q", got, err, test.want)
			}
		})
	}
}

func TestSubdomainScopeAndLabelBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, domain string
		want         bool
	}{
		{name: "ICANN单标签com", domain: "com", want: false},
		{name: "ICANN单标签io", domain: "io", want: false},
		{name: "ICANN多标签", domain: "co.uk", want: false},
		{name: "PRIVATE后缀", domain: "github.io", want: false},
		{name: "另一PRIVATE后缀", domain: "blogspot.com", want: false},
		{name: "合法PRIVATE租户", domain: "tenant.github.io", want: true},
		{name: "合法注册域名", domain: "example.com", want: true},
		{name: "内部单标签", domain: "intranet", want: true},
		{name: "内部多标签", domain: "corp.internal", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := target.SubdomainsAllowed(test.domain); got != test.want {
				t.Fatalf("泛子域范围可用 = %t，预期 %t", got, test.want)
			}
		})
	}
	for _, test := range []struct {
		name, host string
		include    bool
		want       bool
	}{
		{name: "精确本域", host: "example.com", want: true},
		{name: "精确不能借子域", host: "a.example.com", want: false},
		{name: "显式包含子域", host: "a.example.com", include: true, want: true},
		{name: "多级子域", host: "a.b.example.com", include: true, want: true},
		{name: "拼接前缀不是标签", host: "badexample.com", include: true, want: false},
		{name: "规则成为攻击域前缀", host: "example.com.attacker.invalid", include: true, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := target.DomainMatches(test.host, "example.com", test.include); got != test.want {
				t.Fatalf("域名标签匹配 = %t，预期 %t", got, test.want)
			}
		})
	}
}

func TestNormalizeAddressContract(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, kind, input, want string
		invalid                 bool
	}{
		{name: "IPv4", kind: "ip", input: "192.0.2.1", want: "192.0.2.1"},
		{name: "等价IPv6", kind: "ip", input: "2001:0DB8:0:0:0:0:0:1", want: "2001:db8::1"},
		{name: "mapped统一IPv4", kind: "ip", input: "::ffff:192.0.2.1", want: "192.0.2.1"},
		{name: "mapped十六进制统一IPv4", kind: "ip", input: "::FFFF:C000:201", want: "192.0.2.1"},
		{name: "zone禁止", kind: "ip", input: "fe80::1%eth0", invalid: true},
		{name: "缩写IPv4禁止", kind: "ip", input: "127.1", invalid: true},
		{name: "前导零IPv4禁止", kind: "ip", input: "192.000.002.001", invalid: true},
		{name: "整数IPv4禁止", kind: "ip", input: "2130706433", invalid: true},
		{name: "十六进制IPv4禁止", kind: "ip", input: "0x7f000001", invalid: true},
		{name: "IPv4含端口禁止", kind: "ip", input: "192.0.2.1:443", invalid: true},
		{name: "IPv6方括号不是IP", kind: "ip", input: "[2001:db8::1]", invalid: true},
		{name: "标准IPv4范围", kind: "cidr", input: "192.0.2.0/24", want: "192.0.2.0/24"},
		{name: "IPv4单主机范围", kind: "cidr", input: "192.0.2.1/32", want: "192.0.2.1/32"},
		{name: "IPv6单主机范围", kind: "cidr", input: "2001:0DB8::1/128", want: "2001:db8::1/128"},
		{name: "标准IPv6范围", kind: "cidr", input: "2001:0DB8::/32", want: "2001:db8::/32"},
		{name: "IPv4主机位拒绝", kind: "cidr", input: "192.0.2.1/24", invalid: true},
		{name: "IPv6主机位拒绝", kind: "cidr", input: "2001:db8::1/64", invalid: true},
		{name: "mapped前缀拒绝", kind: "cidr", input: "::ffff:192.0.2.0/120", invalid: true},
		{name: "宽mapped前缀拒绝", kind: "cidr", input: "::ffff:0:0/96", invalid: true},
		{name: "CIDR zone拒绝", kind: "cidr", input: "fe80::%eth0/64", invalid: true},
		{name: "CIDR不是资产IP", kind: "ip", input: "192.0.2.0/24", invalid: true},
		{name: "未知类型默认拒绝", kind: "root_domain", input: "example.com", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := target.Normalize(test.kind, test.input)
			if test.invalid {
				if err == nil || got != "" {
					t.Fatalf("无效地址应拒绝且无规范值，得到 %q / %v", got, err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("规范地址 = %q / %v，预期 %q", got, err, test.want)
			}
		})
	}
	if _, err := target.CIDR("192.0.2.9/24"); err == nil || !strings.Contains(err.Error(), "192.0.2.0/24") {
		t.Fatal("CIDR 主机位拒绝必须说明可供用户确认的 masked 范围")
	}
}

func TestNormalizeURLContract(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, input, want string
		invalid           bool
	}{
		{name: "scheme默认HTTP端口与空路径", input: "HTTP://EXAMPLE.COM:80", want: "http://example.com/"},
		{name: "默认HTTPS端口", input: "https://EXAMPLE.COM.:443/a", want: "https://example.com/a"},
		{name: "非默认端口保持", input: "https://example.com:8443/A/", want: "https://example.com:8443/A/"},
		{name: "URL中文host复用IDNA", input: "https://例子.测试", want: "https://xn--fsqu00a.xn--0zwm56d/"},
		{name: "URL mapped host复用IP", input: "http://[::ffff:192.0.2.1]:80", want: "http://192.0.2.1/"},
		{name: "IPv6 host保留括号", input: "https://[2001:0DB8::1]:443", want: "https://[2001:db8::1]/"},
		{name: "只统一path转义hex", input: "https://example.com/%7e?q=%7e", want: "https://example.com/%7E?q=%7e"},
		{name: "转义不解码为字面A", input: "https://example.com/%41", want: "https://example.com/%41"},
		{name: "query顺序与重复项保持", input: "https://example.com/?a=2&a=1", want: "https://example.com/?a=2&a=1"},
		{name: "query加号与空格转义保持", input: "https://example.com/?a=+&b=%20", want: "https://example.com/?a=+&b=%20"},
		{name: "空query标记保持", input: "https://example.com?", want: "https://example.com/?"},
		{name: "重复slash与尾slash保持", input: "https://example.com/a//b/", want: "https://example.com/a//b/"},
		{name: "双重编码不擅自decode", input: "https://example.com/%252e", want: "https://example.com/%252e"},
		{name: "query编码slash不变", input: "https://example.com/?p=%2f", want: "https://example.com/?p=%2f"},
		{name: "URL原始换行不能隐藏", input: "\nhttps://example.com/", invalid: true},
		{name: "host校验错误不能被parse局部err遮蔽", input: "https://bad_host.invalid/", invalid: true},
		{name: "空host标签", input: "https://a..example.com/", invalid: true},
		{name: "未知scheme", input: "ftp://example.com/a", invalid: true},
		{name: "相对URL", input: "/example.com/a", invalid: true},
		{name: "opaqueURL", input: "https:example.com", invalid: true},
		{name: "userinfo禁止", input: "https://user:password@example.com/a", invalid: true},
		{name: "空fragment也禁止", input: "https://example.com/a#", invalid: true},
		{name: "fragment禁止", input: "https://example.com/a#part", invalid: true},
		{name: "原始反斜杠", input: "https://example.com/a\\b", invalid: true},
		{name: "编码slash小写", input: "https://example.com/a%2fb", invalid: true},
		{name: "编码slash大写", input: "https://example.com/a%2Fb", invalid: true},
		{name: "编码backslash", input: "https://example.com/a%5cb", invalid: true},
		{name: "literal点段", input: "https://example.com/a/../b", invalid: true},
		{name: "literal单点段", input: "https://example.com/./b", invalid: true},
		{name: "编码单点段", input: "https://example.com/%2e/b", invalid: true},
		{name: "混合编码双点段", input: "https://example.com/.%2E/b", invalid: true},
		{name: "编码控制字符", input: "https://example.com/%00", invalid: true},
		{name: "非法path百分号", input: "https://example.com/%xx", invalid: true},
		{name: "非法query百分号", input: "https://example.com/?a=%xx", invalid: true},
		{name: "空端口", input: "https://example.com:/a", invalid: true},
		{name: "端口0", input: "https://example.com:0/a", invalid: true},
		{name: "端口超界", input: "https://example.com:65536/a", invalid: true},
		{name: "前导零端口", input: "https://example.com:0443/a", invalid: true},
		{name: "URL zone地址禁止", input: "https://[fe80::1%25eth0]/", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, kind := range []string{"url", "url_exact"} {
				got, err := target.Normalize(kind, test.input)
				if test.invalid {
					if err == nil || got != "" {
						t.Fatalf("%s 无效URL应拒绝且无规范值，得到 %q / %v", kind, got, err)
					}
					continue
				}
				if err != nil || got != test.want {
					t.Fatalf("%s 规范URL = %q / %v，预期 %q", kind, got, err, test.want)
				}
			}
		})
	}
}

func TestStructuredMatchingContract(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, kind, rule, targetKind, value string
		include, want                       bool
	}{
		{name: "域名不授权直接IP", kind: "domain", rule: "example.com", include: true, targetKind: "ip", value: "192.0.2.1"},
		{name: "域名不授权URL所有端点", kind: "domain", rule: "example.com", include: true, targetKind: "url", value: "https://example.com/"},
		{name: "IP精确匹配", kind: "ip", rule: "192.0.2.1", targetKind: "ip", value: "192.0.2.1", want: true},
		{name: "IP不借域名", kind: "ip", rule: "192.0.2.1", targetKind: "domain", value: "192.0.2.1"},
		{name: "CIDR起点", kind: "cidr", rule: "192.0.2.0/24", targetKind: "ip", value: "192.0.2.0", want: true},
		{name: "CIDR终点", kind: "cidr", rule: "192.0.2.0/24", targetKind: "ip", value: "192.0.2.255", want: true},
		{name: "CIDR越界", kind: "cidr", rule: "192.0.2.0/24", targetKind: "ip", value: "192.0.3.0"},
		{name: "CIDR mapped同家族", kind: "cidr", rule: "192.0.2.0/24", targetKind: "ip", value: "::ffff:192.0.2.1", want: true},
		{name: "CIDR异家族", kind: "cidr", rule: "2001:db8::/32", targetKind: "ip", value: "192.0.2.1"},
		{name: "IPv6CIDR边界", kind: "cidr", rule: "2001:db8::/32", targetKind: "ip", value: "2001:db8:ffff:ffff:ffff:ffff:ffff:ffff", want: true},
		{name: "CIDR非法目标", kind: "cidr", rule: "192.0.2.0/24", targetKind: "ip", value: "invalid"},
		{name: "未知规则默认拒绝", kind: "regex", rule: ".*", targetKind: "domain", value: "example.com"},
		{name: "URL exact", kind: "url_exact", rule: "https://example.com/a?a=1&a=2", targetKind: "url", value: "https://example.com/a?a=1&a=2", want: true},
		{name: "URL query顺序不同", kind: "url_exact", rule: "https://example.com/a?a=1&a=2", targetKind: "url", value: "https://example.com/a?a=2&a=1"},
		{name: "URL scheme不同", kind: "url_exact", rule: "https://example.com/a", targetKind: "url", value: "http://example.com/a"},
		{name: "URL端口不同", kind: "url_exact", rule: "https://example.com/a", targetKind: "url", value: "https://example.com:8443/a"},
		{name: "URL path大小写不同", kind: "url_exact", rule: "https://example.com/a", targetKind: "url", value: "https://example.com/A"},
		{name: "URL尾slash不同", kind: "url_exact", rule: "https://example.com/a", targetKind: "url", value: "https://example.com/a/"},
		{name: "URL不能前缀匹配", kind: "url_exact", rule: "https://example.com/a", targetKind: "url", value: "https://example.com/ab"},
		{name: "URL空query不同", kind: "url_exact", rule: "https://example.com/", targetKind: "url", value: "https://example.com/?"},
		{name: "URL转义不等字面", kind: "url_exact", rule: "https://example.com/%41", targetKind: "url", value: "https://example.com/A"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := target.Matches(test.kind, test.rule, test.include, test.targetKind, test.value); got != test.want {
				t.Fatalf("结构化匹配 = %t，预期 %t", got, test.want)
			}
		})
	}
}

func TestReservedTargetContract(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, kind, value string
		want              bool
	}{
		{name: "IPv4 unspecified", kind: "ip", value: "0.0.0.0", want: true},
		{name: "IPv4 loopback全段", kind: "ip", value: "127.42.0.9", want: true},
		{name: "mapped loopback", kind: "ip", value: "::ffff:127.0.0.1", want: true},
		{name: "IPv4 linklocal与metadata", kind: "ip", value: "169.254.169.254", want: true},
		{name: "IPv4 multicast", kind: "ip", value: "224.0.0.1", want: true},
		{name: "平台metadata地址", kind: "ip", value: "100.100.100.200", want: true},
		{name: "IPv6 unspecified", kind: "ip", value: "::", want: true},
		{name: "IPv6 loopback", kind: "ip", value: "::1", want: true},
		{name: "IPv6 linklocal", kind: "ip", value: "fe80::1", want: true},
		{name: "IPv6 multicast", kind: "ip", value: "ff02::1", want: true},
		{name: "IPv6 metadata", kind: "ip", value: "fd00:ec2::254", want: true},
		{name: "localhost标签", kind: "domain", value: "localhost", want: true},
		{name: "localhost子域标签", kind: "domain", value: "a.localhost", want: true},
		{name: "metadata域名", kind: "domain", value: "metadata.google.internal", want: true},
		{name: "域名子串不能硬拒绝", kind: "domain", value: "badlocalhost.invalid", want: false},
		{name: "私网并非保留段", kind: "ip", value: "10.42.0.9", want: false},
		{name: "另一私网并非保留段", kind: "ip", value: "192.168.1.1", want: false},
		{name: "IPv6私网并非保留段", kind: "ip", value: "fd42::9", want: false},
		{name: "文档地址可本地匹配", kind: "ip", value: "192.0.2.1", want: false},
		{name: "URL同样保护localhost", kind: "url", value: "http://localhost/a", want: true},
		{name: "URL同样保护mapped", kind: "url", value: "http://[::ffff:127.0.0.1]/a", want: true},
		{name: "无效IP保守拒绝", kind: "ip", value: "invalid", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := target.Reserved(test.kind, test.value); got != test.want {
				t.Fatalf("平台保留目标 = %t，预期 %t", got, test.want)
			}
		})
	}
}

func TestURLHostUsesCanonicalHostAndEffectivePort(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		value, kind, host string
		port              int
	}{
		{value: "https://example.com/a", kind: "domain", host: "example.com", port: 443},
		{value: "http://example.com/a", kind: "domain", host: "example.com", port: 80},
		{value: "https://192.0.2.1:8443/a", kind: "ip", host: "192.0.2.1", port: 8443},
		{value: "https://[2001:db8::1]/a", kind: "ip", host: "2001:db8::1", port: 443},
	} {
		t.Run(test.value, func(t *testing.T) {
			kind, host, port := target.URLHost(test.value)
			if kind != test.kind || host != test.host || port != test.port {
				t.Fatalf("URL主机 = %q/%q/%d，预期 %q/%q/%d", kind, host, port, test.kind, test.host, test.port)
			}
		})
	}
}
