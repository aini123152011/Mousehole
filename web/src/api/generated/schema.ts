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
            capabilities: ("project:read" | "task_draft:create")[];
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
            code: "invalid_request" | "unauthenticated" | "session_expired" | "identity_not_provisioned" | "csrf_invalid" | "project_forbidden" | "origin_forbidden" | "command_not_found" | "idempotency_conflict" | "request_too_large" | "unsupported_media_type" | "service_unavailable" | "internal_error" | "route_not_found" | "method_not_allowed";
            message: string;
            request_id: components["schemas"]["UUID"];
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
}
