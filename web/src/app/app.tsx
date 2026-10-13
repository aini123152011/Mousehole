import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { BrowserRouter, Link, NavLink, Route, Routes, useLocation } from 'react-router-dom';
import {
  ArrowUpRight,
  Check,
  CircleHelp,
  FilePlus2,
  Layers3,
  Loader2,
  LogIn,
  LogOut,
  Database,
  Shield,
  ListChecks,
} from 'lucide-react';
import { CommandAPIError, getSession, logout, setSessionMemory } from '../api/client';
import type { Session } from '../api/client';
import { Button } from '../components/ui/button';
import { CommandWorkspace } from '../features/commands/command-workspace';
import { useCommandWorkspace } from '../features/commands/use-command-workspace';
import type { components } from '../api/generated/schema';
import { AssetsWorkspace } from '../features/assets/assets-workspace';
import { useAssetSearchState } from '../features/assets/use-asset-search-state';
import { AuthorizationWorkspace } from '../features/authorization/authorization-workspace';
import { PrechecksWorkspace } from '../features/prechecks/prechecks-workspace';
import { useScopeCommand } from '../features/scope/use-scope-command';
import { ScopeCommandPanel } from '../features/scope/scope-command-panel';
import { clearProtectedCache } from './protected-cache';

/** 项目路由共享未知范围命令状态；跳转页面不会丢弃原 ID 或解锁绑定参数。 */
function WorkspaceRoutes({
  actorId,
  project,
}: {
  actorId: string;
  project: components['schemas']['ProjectAccess'];
}) {
  const command = useScopeCommand(actorId, project.id);
  const draftCommand = useCommandWorkspace(actorId, project.id);
  const assetSearch = useAssetSearchState();
  const location = useLocation();
  return (
    <>
      <Routes>
        <Route path="/" element={<CommandWorkspace project={project} workspace={draftCommand} />} />
        <Route
          path="/assets/:assetId?"
          element={
            <AssetsWorkspace
              actorId={actorId}
              project={project}
              command={command}
              searchState={assetSearch}
            />
          }
        />
        <Route
          path="/authorization/:versionId?"
          element={<AuthorizationWorkspace actorId={actorId} project={project} command={command} />}
        />
        <Route
          path="/prechecks/:decisionId?"
          element={<PrechecksWorkspace actorId={actorId} project={project} command={command} />}
        />
        <Route
          path="*"
          element={
            <div className="connection-panel">
              <p>页面不存在。</p>
              <Link to="/">返回任务工作台</Link>
            </div>
          }
        />
      </Routes>
      {location.pathname !== '/' && <ScopeCommandPanel workspace={command} />}
    </>
  );
}

/** 组合企业会话、项目选择和真实命令工作台；会话只来自服务端 Query。 */
export function App() {
  return (
    <BrowserRouter>
      <AppContent />
    </BrowserRouter>
  );
}

/** 服务器会话为唯一身份权威，项目改变时整套工作台重新挂载并清除保护缓存。 */
function AppContent() {
  const queryClient = useQueryClient();
  const location = useLocation();
  const section = location.pathname.startsWith('/assets')
    ? '资产'
    : location.pathname.startsWith('/authorization')
      ? '授权范围'
      : location.pathname.startsWith('/prechecks')
        ? '策略预检查'
        : '任务工作台';
  const [projectId, setProjectId] = useState<string | null>(null);
  const previousActor = useRef<string | null>(null);
  const session = useQuery({
    queryKey: ['session'],
    queryFn: ({ signal }) => getSession(signal),
    retry: false,
  });
  const currentSession: Session | undefined = session.error ? undefined : session.data;
  useEffect(() => {
    setSessionMemory(currentSession ?? null);
    if (
      (session.isError && !session.isPending) ||
      (previousActor.current !== null && previousActor.current !== currentSession?.actor_id)
    )
      void clearProtectedCache(queryClient);
    previousActor.current = currentSession?.actor_id ?? null;
  }, [currentSession, queryClient, session.isError, session.isPending]);

  async function signOut() {
    try {
      await logout();
    } finally {
      setSessionMemory(null);
      await clearProtectedCache(queryClient);
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
              void clearProtectedCache(queryClient);
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
          <NavLink className={({ isActive }) => `nav-link${isActive ? ' active' : ''}`} to="/" end>
            <FilePlus2 size={18} aria-hidden="true" />
            任务工作台
            <ArrowUpRight size={14} className="nav-arrow" aria-hidden="true" />
          </NavLink>
          <NavLink
            className={({ isActive }) => `nav-link${isActive ? ' active' : ''}`}
            to="/assets"
          >
            <Database size={18} aria-hidden="true" />
            资产
          </NavLink>
          <NavLink
            className={({ isActive }) => `nav-link${isActive ? ' active' : ''}`}
            to="/authorization"
          >
            <Shield size={18} aria-hidden="true" />
            授权范围
          </NavLink>
          <NavLink
            className={({ isActive }) => `nav-link${isActive ? ' active' : ''}`}
            to="/prechecks"
          >
            <ListChecks size={18} aria-hidden="true" />
            策略预检查
          </NavLink>
        </nav>
        <div className="sidebar-bottom">
          <span className="local-dot" />
          <div>
            <strong>{currentSession ? '已连接企业身份' : '等待企业登录'}</strong>
            <p>项目数据与命令受成员权限保护</p>
          </div>
        </div>
      </aside>

      <div className="workspace-shell">
        <header className="topbar">
          <div className="breadcrumbs">
            <span>{selectedProject?.name ?? '企业工作空间'}</span>
            <span aria-hidden="true">/</span>
            <strong>{section}</strong>
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
                <span /> {section}
              </div>
              <h1>
                {section === '任务工作台'
                  ? '让每一次提交，都有据可查'
                  : section === '资产'
                    ? '整理资产，保留每一份来源'
                    : section === '授权范围'
                      ? '让授权范围明确，版本可追溯'
                      : '先理解规则，再确认执行条件'}
                <span>。</span>
              </h1>
              <p>
                {section === '任务工作台'
                  ? '先建立清晰的任务草稿，再从同一个命令 ID 确认受理结果。'
                  : section === '资产'
                    ? '统一规范值和项目可见性；资产登记不扩大授权。'
                    : section === '授权范围'
                      ? '已保存正文不可变，候选与激活分离；当前禁止始终优先。'
                      : '静态匹配仍需运行时检查，历史结果不是执行许可。'}
              </p>
            </div>
            <span className="page-label">
              <Check size={14} aria-hidden="true" />
              {section === '任务工作台' ? '草稿工作台' : '项目范围工作台'}
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
            <WorkspaceRoutes
              key={`${currentSession.actor_id}:${selectedProject.id}`}
              actorId={currentSession.actor_id}
              project={selectedProject}
            />
          )}
          <footer className="workspace-footer">
            <span>Mousehole · 从确定的状态开始</span>
            <span>资产与授权 / 0.3</span>
          </footer>
        </main>
      </div>
    </div>
  );
}
