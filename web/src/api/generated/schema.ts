// 此文件由 pnpm run generate 生成，请修改 api/openapi.yaml。
export interface paths {
    "/healthz": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * 检查进程存活
         * @description 只检查 HTTP 进程存活，不表示数据库可用或迁移已完成。
         */
        get: operations["getHealth"];
        put?: never;
        post?: never;
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
        /**
         * 查询本地开发身份
         * @description 返回固定演示身份和唯一允许的项目，不创建登录会话。
         */
        get: operations["getDevelopmentSession"];
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
            header: {
                /** @description 显式声明本地开发协议，不具备凭据作用。 */
                "X-Mousehole-Development": components["parameters"]["Development"];
            };
            path: {
                /** @description 固定本地项目 ID；其他合法 UUID 返回 project_forbidden。 */
                project_id: components["parameters"]["ProjectID"];
                /** @description 客户端发送前生成的稳定命令 ID，查询恢复时沿用。 */
                command_id: components["parameters"]["CommandID"];
            };
            cookie?: never;
        };
        /**
         * 查询原命令受理结果
         * @description 每次查询重新校验开发身份和项目权限，返回数据库中的原始回执。
         *     暂未找到不证明并发中的提交未受理；客户端应保留 ID 并允许再次查询。
         */
        get: operations["getCommandReceipt"];
        /**
         * 受理创建任务草稿命令
         * @description 客户端发送前生成小写 UUID；同项目、ID、身份与规范化参数重复提交返回原回执。
         *     标题去除首尾 Unicode 空白后为 1–120 个 Unicode 字符；不接受未知或重复 JSON 字段。
         *     同 ID 不同参数返回冲突，不修改原记录。受理、草稿、审计和 outbox 同事务。
         *     响应丢失或服务失败时保留原 ID 查询，不自动生成新 ID 重发。
         *     已受理仅表示草稿已持久化，不表示任何目标行为已执行。
         */
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
        /**
         * Format: uuid
         * @description 带连字符的标准小写 UUID。
         */
        UUID: string;
        DevelopmentSession: {
            /** @constant */
            mode: "development";
            /** @constant */
            actor_id: "00000000-0000-4000-8000-000000000001";
            /** @constant */
            project_id: "00000000-0000-4000-8000-000000000002";
            /** @constant */
            project_name: "本地演示项目";
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
            /**
             * @description 已受理，不代表执行完成。
             * @constant
             */
            state: "accepted";
            task_id: components["schemas"]["UUID"];
            task_title: string;
            /** @constant */
            task_state: "draft";
            /**
             * @description 十进制业务版本字符串，避免 JavaScript 整数精度损失。
             * @constant
             */
            version: "1";
            /**
             * Format: date-time
             * @description 原始受理时间，UTC RFC3339，统一使用 Z 时区后缀。
             */
            accepted_at: string;
        };
        APIError: {
            /**
             * @description 稳定机器错误码，禁止通过中文说明分支判断。
             * @enum {string}
             */
            code: "invalid_request" | "project_forbidden" | "development_only" | "command_not_found" | "idempotency_conflict" | "request_too_large" | "unsupported_media_type" | "service_unavailable" | "internal_error" | "route_not_found" | "method_not_allowed";
            /** @description 可直接展示的中文说明，不含内部异常或秘密。 */
            message: string;
            /**
             * Format: uuid
             * @description 服务端生成的请求关联标识。
             */
            request_id: string;
        };
    };
    responses: {
        /** @description 原始命令回执；相同命令回放不新增草稿、审计或 outbox。 */
        Receipt: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["CommandReceipt"];
            };
        };
        /** @description 参数无效，错误码为 invalid_request */
        InvalidRequest: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["APIError"];
            };
        };
        /** @description 开发协议、来源或项目边界拒绝请求 */
        Forbidden: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["APIError"];
            };
        };
        /** @description 内部失败；提交请求应保留原 ID 查询确认 */
        InternalError: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["APIError"];
            };
        };
        /** @description 数据库暂不可用或提交结果未知；请按原命令 ID 查询 */
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
        /** @description 显式声明本地开发协议，不具备凭据作用。 */
        Development: "true";
        /** @description 固定本地项目 ID；其他合法 UUID 返回 project_forbidden。 */
        ProjectID: components["schemas"]["UUID"];
        /** @description 客户端发送前生成的稳定命令 ID，查询恢复时沿用。 */
        CommandID: components["schemas"]["UUID"];
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
            403: components["responses"]["Forbidden"];
        };
    };
    getDevelopmentSession: {
        parameters: {
            query?: never;
            header: {
                /** @description 显式声明本地开发协议，不具备凭据作用。 */
                "X-Mousehole-Development": components["parameters"]["Development"];
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description 本地开发身份 */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["DevelopmentSession"];
                };
            };
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    getCommandReceipt: {
        parameters: {
            query?: never;
            header: {
                /** @description 显式声明本地开发协议，不具备凭据作用。 */
                "X-Mousehole-Development": components["parameters"]["Development"];
            };
            path: {
                /** @description 固定本地项目 ID；其他合法 UUID 返回 project_forbidden。 */
                project_id: components["parameters"]["ProjectID"];
                /** @description 客户端发送前生成的稳定命令 ID，查询恢复时沿用。 */
                command_id: components["parameters"]["CommandID"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            200: components["responses"]["Receipt"];
            400: components["responses"]["InvalidRequest"];
            403: components["responses"]["Forbidden"];
            /** @description 暂未查询到命令，错误码为 command_not_found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            500: components["responses"]["InternalError"];
            503: components["responses"]["Unavailable"];
        };
    };
    acceptCommand: {
        parameters: {
            query?: never;
            header: {
                /** @description 显式声明本地开发协议，不具备凭据作用。 */
                "X-Mousehole-Development": components["parameters"]["Development"];
            };
            path: {
                /** @description 固定本地项目 ID；其他合法 UUID 返回 project_forbidden。 */
                project_id: components["parameters"]["ProjectID"];
                /** @description 客户端发送前生成的稳定命令 ID，查询恢复时沿用。 */
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
            /** @description 命令首次受理，返回持久化回执 */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["CommandReceipt"];
                };
            };
            400: components["responses"]["InvalidRequest"];
            403: components["responses"]["Forbidden"];
            /** @description 同一命令 ID 已绑定不同参数或身份 */
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
            /** @description 请求媒体类型不是 application/json */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["APIError"];
                };
            };
            500: components["responses"]["InternalError"];
            503: components["responses"]["Unavailable"];
        };
    };
}
