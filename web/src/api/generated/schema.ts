// 此文件由 pnpm run generate 生成，请修改 api/openapi.yaml。
export interface paths {
    "/healthz": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** 检查进程存活 */
        get: operations["getHealth"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/login": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * 发起企业 OIDC 登录
         * @description 浏览器导航到此地址；服务端保存一次性 state、nonce 和 PKCE，不返回 token。
         */
        get: operations["startLogin"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/callback": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * 完成企业 OIDC 回调
         * @description 校验 state、登录绑定 Cookie、PKCE、nonce、issuer、签名、audience 和 azp。
         */
        get: operations["completeLogin"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/logout": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** 撤销当前服务端会话 */
        post: operations["logout"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/session": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** 查询当前用户和项目能力 */
        get: operations["getSession"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/projects/{project_id}/commands/{command_id}": {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
                /** @description 客户端生成并在未知结果时保留的稳定命令 UUID。 */
                command_id: components["parameters"]["CommandID"];
            };
            cookie?: never;
        };
        /** 查询当前用户原命令受理结果 */
        get: operations["getCommandReceipt"];
        /** 受理创建任务草稿命令 */
        put: operations["acceptCommand"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/projects/{project_id}/assets": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * 读取项目资产及本项目来源
         * @description 当前活动项目成员可读取；写请求要求 analyst、会话绑定 CSRF 和精确来源。预检查仅本地匹配，历史结果不作为执行许可。
         */
        get: operations["listAssets"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/projects/{project_id}/assets/preview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * 预览资产规范化与整批错误
         * @description 当前活动项目成员可读取；写请求要求 analyst、会话绑定 CSRF 和精确来源。预检查仅本地匹配，历史结果不作为执行许可。
         */
        post: operations["previewAssets"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/projects/{project_id}/scope-commands/{command_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * 按原 ID 查询原身份回执
         * @description 当前活动项目成员可读取；写请求要求 analyst、会话绑定 CSRF 和精确来源。预检查仅本地匹配，历史结果不作为执行许可。
         */
        get: operations["getScopeCommand"];
        /**
         * 受理资产、候选授权或静态预检查
         * @description 当前活动项目成员可读取；写请求要求 analyst、会话绑定 CSRF 和精确来源。预检查仅本地匹配，历史结果不作为执行许可。
         */
        put: operations["acceptScopeCommand"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/projects/{project_id}/authorization-versions": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * 读取授权版本历史
         * @description 当前活动项目成员可读取；写请求要求 analyst、会话绑定 CSRF 和精确来源。预检查仅本地匹配，历史结果不作为执行许可。
         */
        get: operations["listAuthorizationVersions"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/projects/{project_id}/authorization-versions/{version_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * 读取不可变授权版本
         * @description 当前活动项目成员可读取；写请求要求 analyst、会话绑定 CSRF 和精确来源。预检查仅本地匹配，历史结果不作为执行许可。
         */
        get: operations["getAuthorizationVersion"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/projects/{project_id}/policy": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * 读取当前授权指针与禁止规则
         * @description 当前活动项目成员可读取；写请求要求 analyst、会话绑定 CSRF 和精确来源。预检查仅本地匹配，历史结果不作为执行许可。
         */
        get: operations["getProjectPolicy"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/projects/{project_id}/prechecks": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * 读取历史静态决策
         * @description 当前活动项目成员可读取；写请求要求 analyst、会话绑定 CSRF 和精确来源。预检查仅本地匹配，历史结果不作为执行许可。
         */
        get: operations["listPrechecks"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/projects/{project_id}/prechecks/{decision_id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * 读取历史静态决策详情
         * @description 当前活动项目成员可读取；写请求要求 analyst、会话绑定 CSRF 和精确来源。预检查仅本地匹配，历史结果不作为执行许可。
         */
        get: operations["getPrecheck"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        /** Format: uuid */
        UUID: string;
        ProjectAccess: {
            id: components["schemas"]["UUID"];
            name: string;
            /** @enum {string} */
            role: "viewer" | "analyst";
            capabilities: ("project:read" | "task_draft:create" | "asset:import" | "authorization:create" | "policy:precheck")[];
            version: string;
        };
        Session: {
            actor_id: components["schemas"]["UUID"];
            display_name: string;
            /** Format: email */
            email: string | null;
            projects: components["schemas"]["ProjectAccess"][];
            csrf_token: string;
            /** Format: date-time */
            expires_at: string;
        };
        CreateTaskDraftCommand: {
            /** @constant */
            kind: "create_task_draft";
            payload: {
                /** @description 去除首尾 Unicode 空白后为 1–120 个 Unicode 字符。 */
                title: string;
            };
        };
        CommandReceipt: {
            command_id: components["schemas"]["UUID"];
            project_id: components["schemas"]["UUID"];
            /** @constant */
            kind: "create_task_draft";
            /** @constant */
            state: "accepted";
            task_id: components["schemas"]["UUID"];
            task_title: string;
            /** @constant */
            task_state: "draft";
            /** @constant */
            version: "1";
            /** Format: date-time */
            accepted_at: string;
        };
        APIError: {
            /** @enum {string} */
            code: "invalid_request" | "unauthenticated" | "session_expired" | "identity_not_provisioned" | "csrf_invalid" | "project_forbidden" | "origin_forbidden" | "command_not_found" | "idempotency_conflict" | "request_too_large" | "unsupported_media_type" | "service_unavailable" | "internal_error" | "route_not_found" | "method_not_allowed" | "forbidden" | "not_found" | "command_conflict" | "policy_revision_conflict" | "unavailable";
            message: string;
            request_id: components["schemas"]["UUID"];
            /** @description 仅范围 invalid_request 可提供；字段使用 payload.entries.0.value 等固定点路径，消息为受控中文规范化说明，不含原始请求或依赖错误。 */
            validation_errors?: {
                field: string;
                message: string;
            }[];
        };
        AssetInput: {
            /** @enum {string} */
            kind: "domain" | "ip" | "url";
            value: string;
        };
        AssetImportPayload: {
            entries: components["schemas"]["AssetInput"][];
            /** @enum {string} */
            source: "manual" | "api_import";
            notes: string;
            external_ref: string;
        };
        AssetSource: {
            command_id: components["schemas"]["UUID"];
            /** @enum {string} */
            source: "manual" | "api_import";
            notes: string;
            external_ref: string;
            /** Format: date-time */
            created_at: string;
        };
        Asset: {
            id: components["schemas"]["UUID"];
            project_id: components["schemas"]["UUID"];
            /** @enum {string} */
            kind: "domain" | "ip" | "url";
            canonical_value: string;
            /** @constant */
            algorithm_version: "target-v1";
            /** Format: date-time */
            created_at: string;
            sources: components["schemas"]["AssetSource"][];
        };
        AssetPreviewRow: {
            row: number;
            kind: string;
            canonical_value: string;
            error: string;
        };
        AssetPreview: {
            rows: components["schemas"]["AssetPreviewRow"][];
            valid: boolean;
            input_count: string;
            unique_count: string;
            duplicate_count: string;
            existing_count: string;
        };
        ScopeEntry: {
            /** @enum {string} */
            kind: "domain" | "ip" | "cidr" | "url_exact";
            value: string;
            include_subdomains: boolean;
            operations: ("dns_lookup" | "tcp_connect" | "http_metadata")[];
            ports: number[];
            resolved_ip_cidrs: string[];
        };
        AuthorizationBudget: {
            max_requests: string;
            max_concurrency: string;
            max_requests_per_second: string;
        };
        AuthorizationContent: {
            title: string;
            basis_ref: string;
            contact: string;
            /** Format: date-time */
            valid_from: string;
            /** Format: date-time */
            expires_at: string;
            /** @constant */
            algorithm_version: "target-v1";
            /** @constant */
            policy_version: "policy-v1";
            /** @constant */
            catalog_version: "precheck-v1";
            /** @constant */
            max_operation_level: "non_destructive";
            budget: components["schemas"]["AuthorizationBudget"];
            /** @constant */
            redirect_policy: "deny";
            /** @constant */
            credential_policy: "none";
            entries: components["schemas"]["ScopeEntry"][];
        };
        AuthorizationVersion: {
            id: components["schemas"]["UUID"];
            project_id: components["schemas"]["UUID"];
            sequence: string;
            content_hash: string;
            content: components["schemas"]["AuthorizationContent"];
            entry_ids: components["schemas"]["UUID"][];
            /** @enum {string} */
            state: "candidate" | "active" | "superseded" | "revoked";
            /** @enum {string} */
            display_state: "candidate" | "active" | "superseded" | "revoked" | "expired" | "not_yet_valid";
            /** Format: date-time */
            created_at: string;
        };
        PrecheckParameters: {
            port?: number;
        };
        PrecheckPayload: {
            authorization_version_id: components["schemas"]["UUID"];
            /** @enum {string} */
            operation: "dns_lookup" | "tcp_connect" | "http_metadata";
            target: string;
            parameters: components["schemas"]["PrecheckParameters"];
        };
        RuntimeCheck: {
            code: string;
            /** @enum {string} */
            status: "required" | "not_applicable";
            message: string;
        };
        PolicyDecision: {
            id: components["schemas"]["UUID"];
            project_id: components["schemas"]["UUID"];
            command_id: components["schemas"]["UUID"];
            authorization_version_id: components["schemas"]["UUID"];
            /** @enum {string} */
            operation: "dns_lookup" | "tcp_connect" | "http_metadata";
            canonical_target: string;
            parameters: components["schemas"]["PrecheckParameters"];
            /** @enum {string} */
            outcome: "deny" | "requires_runtime_checks";
            reason_codes: string[];
            message: string;
            matched_entry_id: components["schemas"]["UUID"] | null;
            /** Format: date-time */
            checked_at: string;
            content_hash: string;
            global_revision: string;
            project_revision: string;
            /** @constant */
            algorithm_version: "target-v1";
            /** @constant */
            policy_version: "policy-v1";
            /** @constant */
            catalog_version: "precheck-v1";
            runtime_checks: components["schemas"]["RuntimeCheck"][];
        };
        ScopeDeny: {
            id: components["schemas"]["UUID"];
            project_id: components["schemas"]["UUID"] | null;
            /** @enum {string} */
            kind: "domain" | "ip" | "cidr" | "url_exact";
            value: string;
            include_subdomains: boolean;
            reason: string;
            active: boolean;
            /** Format: date-time */
            created_at: string;
        };
        ProjectPolicy: {
            project_id: components["schemas"]["UUID"];
            global_revision: string;
            project_revision: string;
            active_version_id: components["schemas"]["UUID"] | null;
            denies: components["schemas"]["ScopeDeny"][];
        };
        ImportAssetsCommand: {
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            kind: "import_assets";
            payload: components["schemas"]["AssetImportPayload"];
        };
        CreateAuthorizationVersionCommand: {
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            kind: "create_authorization_version";
            payload: components["schemas"]["AuthorizationContent"];
        };
        PrecheckTargetCommand: {
            /**
             * @description discriminator enum property added by openapi-typescript
             * @enum {string}
             */
            kind: "precheck_target";
            payload: components["schemas"]["PrecheckPayload"];
        };
        ScopeCommandRequest: components["schemas"]["ImportAssetsCommand"] | components["schemas"]["CreateAuthorizationVersionCommand"] | components["schemas"]["PrecheckTargetCommand"];
        AssetImportResult: {
            /** @constant */
            type: "assets_imported";
            input_count: string;
            unique_count: string;
            duplicate_count: string;
            existing_count: string;
            assets: components["schemas"]["Asset"][];
        };
        AuthorizationVersionResult: {
            /** @constant */
            type: "authorization_version";
            version: components["schemas"]["AuthorizationVersion"];
        };
        PrecheckResult: {
            /** @constant */
            type: "precheck";
            decision: components["schemas"]["PolicyDecision"];
        };
        PolicyChangeResult: {
            /** @constant */
            type: "policy_changed";
            object_id: components["schemas"]["UUID"];
            global_revision: string;
            project_revision: string;
        };
        ScopeCommandReceipt: {
            command_id: components["schemas"]["UUID"];
            project_id: components["schemas"]["UUID"] | null;
            /** @enum {string} */
            kind: "import_assets" | "create_authorization_version" | "precheck_target" | "activate_authorization_version" | "revoke_authorization_version" | "add_scope_deny" | "revoke_scope_deny";
            /** @constant */
            state: "accepted";
            /** Format: date-time */
            accepted_at: string;
            result: components["schemas"]["AssetImportResult"] | components["schemas"]["AuthorizationVersionResult"] | components["schemas"]["PrecheckResult"] | components["schemas"]["PolicyChangeResult"];
        };
        AssetPage: {
            items: components["schemas"]["Asset"][];
            has_more: boolean;
            next_cursor: string | null;
            page_count: string;
        };
        AuthorizationVersionPage: {
            items: components["schemas"]["AuthorizationVersion"][];
            has_more: boolean;
            next_cursor: string | null;
            page_count: string;
        };
        PrecheckPage: {
            items: components["schemas"]["PolicyDecision"][];
            has_more: boolean;
            next_cursor: string | null;
            page_count: string;
        };
    };
    responses: {
        /** @description 原命令持久化回执 */
        Receipt: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["CommandReceipt"];
            };
        };
        /** @description 请求无效 */
        InvalidRequest: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["APIError"];
            };
        };
        /** @description 未登录或会话已失效 */
        Unauthenticated: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["APIError"];
            };
        };
        /** @description 外部身份未预置 */
        IdentityNotProvisioned: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["APIError"];
            };
        };
        /** @description CSRF 或来源校验失败 */
        CSRFInvalid: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["APIError"];
            };
        };
        /** @description 当前 actor 未查询到原命令 */
        CommandNotFound: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["APIError"];
            };
        };
        /** @description 依赖失败或结果待确认 */
        Unavailable: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["APIError"];
            };
        };
    };
    parameters: {
        /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
        ProjectID: components["schemas"]["UUID"];
        /** @description 客户端生成并在未知结果时保留的稳定命令 UUID。 */
        CommandID: components["schemas"]["UUID"];
        /** @description 从当前 session 响应取得的会话绑定 token；不得持久化。 */
        CSRF: string;
    };
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    getHealth: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 进程存活 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        /** @constant */
                        status: "ok";
                    };
                };
            };
        };
    };
    startLogin: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 重定向到 OIDC authorization endpoint */
            302: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["InvalidRequest"];
            503: components["responses"]["Unavailable"];
        };
    };
    completeLogin: {
        parameters: {
            query?: {
                code?: string;
                state?: string;
                error?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 登录成功并重定向到 Web 应用 */
            303: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            /** @description 登录尝试无效或已消费；返回不含 code、token 或 claims 的中文安全页 */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "text/html": string;
                };
            };
            /** @description 外部身份未预置；返回中文安全页和 request_id */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "text/html": string;
                };
            };
            /** @description 身份依赖暂不可用；返回不含底层错误的中文安全页 */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "text/html": string;
                };
            };
        };
    };
    logout: {
        parameters: {
            query?: never;
            header: {
                /** @description 从当前 session 响应取得的会话绑定 token；不得持久化。 */
                "X-CSRF-Token": components["parameters"]["CSRF"];
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 会话已撤销并清除 Cookie */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["CSRFInvalid"];
        };
    };
    getSession: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 当前会话投影和内存 CSRF token */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Session"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            503: components["responses"]["Unavailable"];
        };
    };
    getCommandReceipt: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
                /** @description 客户端生成并在未知结果时保留的稳定命令 UUID。 */
                command_id: components["parameters"]["CommandID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            200: components["responses"]["Receipt"];
            401: components["responses"]["Unauthenticated"];
            /** @description 项目无权访问 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            404: components["responses"]["CommandNotFound"];
            503: components["responses"]["Unavailable"];
        };
    };
    acceptCommand: {
        parameters: {
            query?: never;
            header: {
                /** @description 从当前 session 响应取得的会话绑定 token；不得持久化。 */
                "X-CSRF-Token": components["parameters"]["CSRF"];
            };
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
                /** @description 客户端生成并在未知结果时保留的稳定命令 UUID。 */
                command_id: components["parameters"]["CommandID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreateTaskDraftCommand"];
            };
        };
        responses: {
            200: components["responses"]["Receipt"];
            /** @description 首次受理 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CommandReceipt"];
                };
            };
            400: components["responses"]["InvalidRequest"];
            401: components["responses"]["Unauthenticated"];
            /** @description 项目成员、角色或 CSRF 校验拒绝 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            404: components["responses"]["CommandNotFound"];
            /** @description 同 ID 参数冲突 */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 请求体超过 16 KiB */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 媒体类型错误 */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            503: components["responses"]["Unavailable"];
        };
    };
    listAssets: {
        parameters: {
            query?: {
                page_size?: number;
                cursor?: string;
                kind?: "domain" | "ip" | "url";
                search?: string;
            };
            header?: never;
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 读取项目资产及本项目来源 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AssetPage"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
        };
    };
    previewAssets: {
        parameters: {
            query?: never;
            header: {
                /** @description 从当前 session 响应取得的会话绑定 token；不得持久化。 */
                "X-CSRF-Token": components["parameters"]["CSRF"];
            };
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["AssetImportPayload"];
            };
        };
        responses: {
            /** @description 预览资产规范化与整批错误 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AssetPreview"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
        };
    };
    getScopeCommand: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
                /** @description 客户端生成并在未知结果时保留的稳定命令 UUID。 */
                command_id: components["parameters"]["CommandID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 按原 ID 查询原身份回执 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ScopeCommandReceipt"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
        };
    };
    acceptScopeCommand: {
        parameters: {
            query?: never;
            header: {
                /** @description 从当前 session 响应取得的会话绑定 token；不得持久化。 */
                "X-CSRF-Token": components["parameters"]["CSRF"];
            };
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
                /** @description 客户端生成并在未知结果时保留的稳定命令 UUID。 */
                command_id: components["parameters"]["CommandID"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ScopeCommandRequest"];
            };
        };
        responses: {
            /** @description 受理资产、候选授权或静态预检查 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ScopeCommandReceipt"];
                };
            };
            /** @description 受理资产、候选授权或静态预检查 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ScopeCommandReceipt"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
        };
    };
    listAuthorizationVersions: {
        parameters: {
            query?: {
                page_size?: number;
                cursor?: string;
            };
            header?: never;
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 读取授权版本历史 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthorizationVersionPage"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
        };
    };
    getAuthorizationVersion: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
                version_id: components["schemas"]["UUID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 读取不可变授权版本 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthorizationVersion"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
        };
    };
    getProjectPolicy: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 读取当前授权指针与禁止规则 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ProjectPolicy"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
        };
    };
    listPrechecks: {
        parameters: {
            query?: {
                page_size?: number;
                cursor?: string;
            };
            header?: never;
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 读取历史静态决策 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PrecheckPage"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
        };
    };
    getPrecheck: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description 项目 UUID；最终权限由当前活动成员关系决定。 */
                project_id: components["parameters"]["ProjectID"];
                decision_id: components["schemas"]["UUID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 读取历史静态决策详情 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PolicyDecision"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            /** @description 中文错误；5xx 结果可能未知，保留原命令 ID 查询。 */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
        };
    };
}
