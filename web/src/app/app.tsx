import { useQuery } from '@tanstack/react-query';
import { ArrowUpRight, Check, CircleHelp, FilePlus2, Layers3, Loader2 } from 'lucide-react';
import { getDevelopmentSession } from '../api/client';
import { Button } from '../components/ui/button';
import { CommandWorkspace } from '../features/commands/command-workspace';

/** 组合本地项目外壳和真实命令工作台，不展示尚未实现的业务入口。 */
export function App() {
  const session = useQuery({
    queryKey: ['development-session'],
    queryFn: ({ signal }) => getDevelopmentSession(signal),
    retry: false,
  });

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
        <div className="workspace-identity">
          <div className="workspace-icon">
            <Layers3 size={18} aria-hidden="true" />
          </div>
          <div>
            <p className="workspace-name">本地工作空间</p>
            <p className="workspace-caption">开发模式 · 固定项目</p>
          </div>
        </div>
        <div className="sidebar-section-label">项目</div>
        <div className="project-switcher">
          <span className="project-initial">M</span>
          <span>{session.data?.project_name ?? '本地演示项目'}</span>
        </div>
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
            <strong>本地开发模式</strong>
            <p>当前仅提供草稿管理</p>
          </div>
        </div>
      </aside>

      <div className="workspace-shell">
        <header className="topbar">
          <div className="breadcrumbs">
            <span>本地演示项目</span>
            <span aria-hidden="true">/</span>
            <strong>任务工作台</strong>
          </div>
          <div className="topbar-identity">
            <span className="avatar">本</span>
            <span>本地开发身份</span>
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
              <span>正在连接本地项目…</span>
            </div>
          ) : session.isError ? (
            <div className="connection-panel connection-error" role="alert">
              <CircleHelp size={22} aria-hidden="true" />
              <div>
                <h2>暂时无法连接本地项目</h2>
                <p>请确认本地 API 已启动，然后重新连接。</p>
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
          ) : (
            <CommandWorkspace session={session.data} />
          )}
          <footer className="workspace-footer">
            <span>Mousehole · 从确定的状态开始</span>
            <span>开发样本 / 0.1</span>
          </footer>
        </main>
      </div>
    </div>
  );
}
