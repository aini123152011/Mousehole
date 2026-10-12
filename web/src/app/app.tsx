import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowUpRight,
  Check,
  CircleHelp,
  FilePlus2,
  Layers3,
  Loader2,
  LogIn,
  LogOut,
} from 'lucide-react';
import { CommandAPIError, getSession, logout, setSessionMemory } from '../api/client';
import type { Session } from '../api/client';
import { Button } from '../components/ui/button';
import { CommandWorkspace } from '../features/commands/command-workspace';

/** 组合企业会话、项目选择和真实命令工作台；会话只来自服务端 Query。 */
export function App() {
  const queryClient = useQueryClient();
  const [projectId, setProjectId] = useState<string | null>(null);
  const session = useQuery({
    queryKey: ['session'],
    queryFn: ({ signal }) => getSession(signal),
    retry: false,
  });
  const currentSession: Session | undefined = session.error ? undefined : session.data;
  useEffect(() => {
    setSessionMemory(currentSession ?? null);
  }, [currentSession]);

  async function signOut() {
    try {
      await logout();
    } finally {
      setSessionMemory(null);
      queryClient.removeQueries({ queryKey: ['commands'] });
      await queryClient.resetQueries({ queryKey: ['session'] });
    }
  }

  const selectedProject =
    currentSession?.projects.find((project) => project.id === projectId) ??
    currentSession?.projects[0] ??
    null;
  const error = session.error instanceof CommandAPIError ? session.error : null;
  const unauthenticated = error?.status === 401;
  const forbidden = error?.code === 'identity_not_provisioned';

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        跳到工作台
      </a>
      <aside className="sidebar" aria-label="项目导航">
        <a className="brand" href="#main-content" aria-label="Mousehole 工作台">
          <span className="brand-mark" aria-hidden="true">
            <span />
          </span>
          <span>
            mousehole<span className="brand-dot">.</span>
          </span>
        </a>
        {currentSession && (
          <div className="workspace-identity">
            <div className="workspace-icon">
              <Layers3 size={18} aria-hidden="true" />
            </div>
            <div>
              <p className="workspace-name">企业工作空间</p>
              <p className="workspace-caption">{currentSession.display_name}</p>
            </div>
          </div>
        )}
        <div className="sidebar-section-label">项目</div>
        {currentSession && currentSession.projects.length > 0 ? (
          <select
            className="project-switcher"
            aria-label="选择项目"
            value={selectedProject?.id ?? ''}
            onChange={(event) => {
              queryClient.removeQueries({ queryKey: ['commands'] });
              setProjectId(event.target.value);
            }}
          >
            {currentSession.projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        ) : (
          <div className="project-switcher">尚未开通项目权限</div>
        )}
        <nav aria-label="项目功能">
          <a className="nav-link active" href="#main-content" aria-current="page">
            <FilePlus2 size={18} aria-hidden="true" />
            任务工作台
            <ArrowUpRight size={14} className="nav-arrow" aria-hidden="true" />
          </a>
        </nav>
        <div className="sidebar-bottom">
          <span className="local-dot" />
          <div>
            <strong>{currentSession ? '已连接企业身份' : '等待企业登录'}</strong>
            <p>草稿命令受项目权限保护</p>
          </div>
        </div>
      </aside>

      <div className="workspace-shell">
        <header className="topbar">
          <div className="breadcrumbs">
            <span>{selectedProject?.name ?? '企业工作空间'}</span>
            <span aria-hidden="true">/</span>
            <strong>任务工作台</strong>
          </div>
          <div className="topbar-identity">
            {currentSession ? (
              <>
                <span className="avatar">{currentSession.display_name.slice(0, 1)}</span>
                <span>{currentSession.display_name}</span>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => void signOut()}
                  aria-label="退出登录"
                >
                  <LogOut size={15} aria-hidden="true" />
                  退出
                </Button>
              </>
            ) : (
              <span>未登录</span>
            )}
          </div>
        </header>
        <main id="main-content" className="main-content" tabIndex={-1}>
          <div className="page-heading">
            <div>
              <div className="eyebrow">
                <span /> 任务工作台
              </div>
              <h1>
                让每一次提交，都有据可查<span>。</span>
              </h1>
              <p>先建立清晰的任务草稿，再从同一个命令 ID 确认受理结果。</p>
            </div>
            <span className="page-label">
              <Check size={14} aria-hidden="true" />
              草稿工作台
            </span>
          </div>
          {session.isPending ? (
            <div className="connection-panel" role="status">
              <Loader2 className="animate-spin" size={20} aria-hidden="true" />
              <span>正在恢复企业会话…</span>
            </div>
          ) : unauthenticated ? (
            <div className="connection-panel connection-error" role="alert">
              <LogIn size={22} aria-hidden="true" />
              <div>
                <h2>请先登录企业身份</h2>
                <p>使用管理员预置的企业账号进入平台。</p>
              </div>
              <Button variant="outline" asChild>
                <a href="/api/v1/auth/login">登录</a>
              </Button>
            </div>
          ) : forbidden ? (
            <div className="connection-panel connection-error" role="alert">
              <CircleHelp size={22} aria-hidden="true" />
              <div>
                <h2>身份尚未开通</h2>
                <p>当前企业身份没有预置项目权限，请联系管理员。</p>
              </div>
            </div>
          ) : session.isError ? (
            <div className="connection-panel connection-error" role="alert">
              <CircleHelp size={22} aria-hidden="true" />
              <div>
                <h2>暂时无法恢复会话</h2>
                <p>请确认 API 已启动，然后重新连接。</p>
              </div>
              <Button
                variant="outline"
                onClick={() => {
                  void session.refetch();
                }}
                disabled={session.isFetching}
              >
                重新连接
              </Button>
            </div>
          ) : !currentSession || !selectedProject ? (
            <div className="connection-panel" role="status">
              <CircleHelp size={22} aria-hidden="true" />
              <span>尚未开通项目权限。</span>
            </div>
          ) : (
            <CommandWorkspace
              key={`${currentSession.actor_id}:${selectedProject.id}`}
              actorId={currentSession.actor_id}
              project={selectedProject}
            />
          )}
          <footer className="workspace-footer">
            <span>Mousehole · 从确定的状态开始</span>
            <span>企业身份 / 0.2</span>
          </footer>
        </main>
      </div>
    </div>
  );
}
