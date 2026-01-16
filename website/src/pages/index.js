import { useState, useCallback, useEffect } from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';

const TABS = ['td', 'git', 'files', 'conversations', 'worktrees'];

const INSTALL_COMMAND = 'curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash';

function CopyButton({ text }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  }, [text]);

  return (
    <button
      type="button"
      className="sc-copyBtn"
      onClick={handleCopy}
      aria-label={copied ? 'Copied' : 'Copy to clipboard'}
    >
      <i className={copied ? 'icon-check' : 'icon-copy'} />
    </button>
  );
}

function TdPane() {
  return (
    <>
      <p className="sc-sectionTitle">Tasks</p>
      <div className="sc-list">
        <div className="sc-item sc-itemActive">
          <span className="sc-bullet sc-bulletGreen" />
          <span>td-a1b2c3 Implement auth flow</span>
        </div>
        <div className="sc-item">
          <span className="sc-bullet sc-bulletBlue" />
          <span>td-d4e5f6 Add rate limiting</span>
        </div>
        <div className="sc-item">
          <span className="sc-bullet sc-bulletPink" />
          <span>td-g7h8i9 Fix memory leak</span>
        </div>
      </div>
      <div style={{ height: 12 }} />
      <div className="sc-codeBlock">
        <div className="sc-lineDim">td-a1b2c3 | in_progress</div>
        <div style={{ height: 6 }} />
        <div><span className="sc-lineGreen">Title:</span> Implement auth flow</div>
        <div><span className="sc-lineBlue">Status:</span> in_progress</div>
        <div><span className="sc-lineYellow">Created:</span> 2h ago</div>
        <div style={{ height: 6 }} />
        <div className="sc-lineDim">Subtasks:</div>
        <div>  <span className="sc-lineGreen">[x]</span> Create auth middleware</div>
        <div>  <span className="sc-lineGreen">[x]</span> Add JWT validation</div>
        <div>  <span className="sc-linePink">[ ]</span> Write integration tests</div>
      </div>
    </>
  );
}

function GitPane() {
  return (
    <>
      <p className="sc-sectionTitle">Changes</p>
      <div className="sc-list">
        <div className="sc-item sc-itemActive">
          <span className="sc-bullet sc-bulletGreen" />
          <span>M internal/auth/middleware.go</span>
        </div>
        <div className="sc-item">
          <span className="sc-bullet sc-bulletGreen" />
          <span>A internal/auth/jwt.go</span>
        </div>
        <div className="sc-item">
          <span className="sc-bullet sc-bulletPink" />
          <span>D internal/auth/old_auth.go</span>
        </div>
      </div>
      <div style={{ height: 12 }} />
      <div className="sc-codeBlock">
        <div className="sc-lineDim">internal/auth/middleware.go</div>
        <div style={{ height: 6 }} />
        <div><span className="sc-lineBlue">@@ -42,6 +42,18 @@</span></div>
        <div><span className="sc-lineGreen">+ func AuthMiddleware(next http.Handler) http.Handler {'{'}</span></div>
        <div><span className="sc-lineGreen">+   return http.HandlerFunc(func(w, r) {'{'}</span></div>
        <div><span className="sc-lineGreen">+     token := r.Header.Get("Authorization")</span></div>
        <div><span className="sc-lineGreen">+     if !ValidateJWT(token) {'{'}</span></div>
        <div><span className="sc-lineGreen">+       http.Error(w, "Unauthorized", 401)</span></div>
        <div><span className="sc-lineGreen">+       return</span></div>
        <div><span className="sc-lineGreen">+     {'}'}</span></div>
        <div><span className="sc-lineGreen">+     next.ServeHTTP(w, r)</span></div>
        <div><span className="sc-lineGreen">+   {'}'})</span></div>
        <div><span className="sc-lineGreen">+ {'}'}</span></div>
      </div>
    </>
  );
}

function FilesPane() {
  return (
    <>
      <p className="sc-sectionTitle">Project Files</p>
      <div className="sc-list">
        <div className="sc-item">
          <span className="sc-lineDim">[v]</span>
          <span>internal/</span>
        </div>
        <div className="sc-item" style={{ paddingLeft: 20 }}>
          <span className="sc-lineDim">[v]</span>
          <span>auth/</span>
        </div>
        <div className="sc-item sc-itemActive" style={{ paddingLeft: 36 }}>
          <span className="sc-bullet sc-bulletBlue" />
          <span>middleware.go</span>
        </div>
        <div className="sc-item" style={{ paddingLeft: 36 }}>
          <span className="sc-bullet sc-bulletBlue" />
          <span>jwt.go</span>
        </div>
        <div className="sc-item" style={{ paddingLeft: 20 }}>
          <span className="sc-lineDim">[&gt;]</span>
          <span>plugins/</span>
        </div>
      </div>
      <div style={{ height: 12 }} />
      <div className="sc-codeBlock">
        <div className="sc-lineDim">middleware.go | 156 lines</div>
        <div style={{ height: 6 }} />
        <div><span className="sc-lineBlue">package</span> auth</div>
        <div style={{ height: 4 }} />
        <div><span className="sc-lineBlue">import</span> (</div>
        <div>  <span className="sc-lineYellow">"net/http"</span></div>
        <div>  <span className="sc-lineYellow">"strings"</span></div>
        <div>)</div>
        <div style={{ height: 4 }} />
        <div><span className="sc-linePink">// AuthMiddleware validates JWT tokens</span></div>
        <div><span className="sc-lineBlue">func</span> <span className="sc-lineGreen">AuthMiddleware</span>(next http.Handler)...</div>
      </div>
    </>
  );
}

function ConversationsPane() {
  return (
    <>
      <p className="sc-sectionTitle">Recent Sessions</p>
      <div className="sc-list">
        <div className="sc-item sc-itemActive">
          <span className="sc-bullet sc-bulletGreen" />
          <span>auth-flow-impl | 24m ago</span>
        </div>
        <div className="sc-item">
          <span className="sc-bullet sc-bulletBlue" />
          <span>fix-rate-limit | 2h ago</span>
        </div>
        <div className="sc-item">
          <span className="sc-bullet sc-bulletPink" />
          <span>refactor-plugins | 1d ago</span>
        </div>
      </div>
      <div style={{ height: 12 }} />
      <div className="sc-codeBlock">
        <div className="sc-lineDim">auth-flow-impl | Claude Code</div>
        <div style={{ height: 6 }} />
        <div><span className="sc-lineBlue">User:</span> Add JWT auth to the API</div>
        <div style={{ height: 4 }} />
        <div><span className="sc-lineGreen">Claude:</span> I'll implement JWT authentication.</div>
        <div className="sc-lineDim">First, let me check the existing auth...</div>
        <div style={{ height: 4 }} />
        <div><span className="sc-lineYellow">-&gt;</span> Read internal/auth/middleware.go</div>
        <div><span className="sc-lineYellow">-&gt;</span> Edit internal/auth/jwt.go</div>
        <div style={{ height: 4 }} />
        <div className="sc-lineDim">12.4k tokens | 24 minutes</div>
      </div>
    </>
  );
}

function WorktreesPane() {
  return (
    <>
      <p className="sc-sectionTitle">Worktrees</p>
      <div className="sc-list">
        <div className="sc-item">
          <span className="sc-bullet sc-bulletGreen" />
          tmux-guide
        </div>
        <div className="sc-item">
          <span className="sc-bullet sc-bulletBlue" />
          waiting-check
        </div>
        <div className="sc-item sc-itemActive">
          <span className="sc-bullet sc-bulletPink" />
          worktree-merge-plan
        </div>
      </div>
      <div style={{ height: 12 }} />
      <div className="sc-codeBlock" role="img" aria-label="Sidecar output pane preview">
        <div className="sc-lineDim">worktree-merge-plan | PR #47</div>
        <div style={{ height: 6 }} />
        <div>
          <span className="sc-lineGreen">+ </span>
          <span className="sc-lineDim">// Option: Keep worktree</span>
        </div>
        <div>
          <span className="sc-lineGreen">+ </span>
          <span className="sc-lineDim">{'if !mergeState.DeleteAfterMerge {'}</span>
        </div>
        <div>
          <span className="sc-lineGreen">+ </span>
          <span className="sc-lineDim">{'  Render("* Keep worktree")'}</span>
        </div>
        <div>
          <span className="sc-linePink">- </span>
          <span className="sc-lineDim">Press Enter to check now</span>
        </div>
        <div>
          <span className="sc-lineGreen">+ </span>
          <span className="sc-lineDim">{'Enter: check | Esc: exit | arrows: navigate'}</span>
        </div>
        <div style={{ height: 6 }} />
        <div className="sc-lineDim">* 3 commits ahead of main</div>
        <div className="sc-lineDim">* Ready to merge</div>
      </div>
    </>
  );
}

function Frame({ activeTab, onTabChange }) {
  const [time, setTime] = useState(() => {
    const now = new Date();
    return now.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false });
  });

  useEffect(() => {
    const updateTime = () => {
      const now = new Date();
      setTime(now.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false }));
    };
    const interval = setInterval(updateTime, 1000);
    return () => clearInterval(interval);
  }, []);

  const renderPane = () => {
    switch (activeTab) {
      case 'td': return <TdPane />;
      case 'git': return <GitPane />;
      case 'files': return <FilesPane />;
      case 'conversations': return <ConversationsPane />;
      case 'worktrees': return <WorktreesPane />;
      default: return <WorktreesPane />;
    }
  };

  return (
    <div className="sc-frame">
      <div className="sc-frameTop">
        <div className="sc-dots" aria-hidden="true">
          <span className="sc-dot" />
          <span className="sc-dot" />
          <span className="sc-dot" />
        </div>
        <div className="sc-topRight">
          <span className="sc-codeInline">sidecar</span>
          <span className="sc-lineDim">{time}</span>
        </div>
      </div>

      <div className="sc-tabs">
        {TABS.map((tab) => (
          <button
            key={tab}
            className={`sc-tab ${activeTab === tab ? 'sc-tabActive' : ''}`}
            onClick={() => onTabChange(tab)}
            type="button"
          >
            {tab}
          </button>
        ))}
      </div>

      <div className="sc-frameBodySingle">
        <div className="sc-pane" key={activeTab}>
          {renderPane()}
        </div>
      </div>

      <div className="sc-frameFooter">
        <span className="sc-lineYellow">tab</span>
        <span className="sc-lineDim"> switch | </span>
        <span className="sc-lineYellow">enter</span>
        <span className="sc-lineDim"> select | </span>
        <span className="sc-lineYellow">?</span>
        <span className="sc-lineDim"> help | </span>
        <span className="sc-lineYellow">q</span>
        <span className="sc-lineDim"> quit</span>
      </div>
    </div>
  );
}

function FeatureCard({ id, title, chip, children, isHighlighted, isHero, onClick }) {
  return (
    <div
      className={`sc-card ${isHero ? 'sc-cardHero' : ''} ${isHighlighted ? 'sc-cardHighlighted' : ''}`}
      onClick={onClick}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => e.key === 'Enter' && onClick?.()}
    >
      <div className="sc-cardHeader">
        <h3 className="sc-cardTitle">{title}</h3>
        <span className="sc-chip">{chip}</span>
      </div>
      <p className="sc-cardBody">{children}</p>
    </div>
  );
}

// Mockup screens for component deep dive
function TdMockup() {
  return (
    <div className="sc-mockup sc-mockupTd">
      <div className="sc-mockupHeader">
        <span className="sc-mockupTitle">Task Management</span>
        <span className="sc-lineDim">3 tasks | 1 in progress</span>
      </div>
      <div className="sc-mockupBody">
        <div className="sc-mockupSidebar">
          <div className="sc-mockupItem sc-mockupItemActive">
            <span className="sc-bullet sc-bulletGreen" />
            <div>
              <div>td-a1b2c3</div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>Implement auth</div>
            </div>
          </div>
          <div className="sc-mockupItem">
            <span className="sc-bullet sc-bulletBlue" />
            <div>
              <div>td-d4e5f6</div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>Rate limiting</div>
            </div>
          </div>
          <div className="sc-mockupItem">
            <span className="sc-bullet sc-bulletPink" />
            <div>
              <div>td-g7h8i9</div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>Memory leak</div>
            </div>
          </div>
        </div>
        <div className="sc-mockupMain">
          <div className="sc-mockupDetail">
            <div className="sc-lineGreen" style={{ fontSize: 14, marginBottom: 8 }}>Implement auth flow</div>
            <div style={{ display: 'grid', gap: 4, fontSize: 12 }}>
              <div><span className="sc-lineDim">Status:</span> <span className="sc-lineYellow">in_progress</span></div>
              <div><span className="sc-lineDim">Created:</span> 2h ago</div>
              <div><span className="sc-lineDim">Epic:</span> td-epic-auth</div>
            </div>
            <div style={{ marginTop: 12, borderTop: '1px solid rgba(255,255,255,0.08)', paddingTop: 12 }}>
              <div className="sc-lineDim" style={{ marginBottom: 6 }}>Subtasks</div>
              <div style={{ display: 'grid', gap: 4, fontSize: 12 }}>
                <div><span className="sc-lineGreen">[x]</span> Create auth middleware</div>
                <div><span className="sc-lineGreen">[x]</span> Add JWT validation</div>
                <div><span className="sc-linePink">[ ]</span> Write integration tests</div>
                <div><span className="sc-linePink">[ ]</span> Update API docs</div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div className="sc-mockupFooter">
        <span className="sc-lineYellow">n</span><span className="sc-lineDim"> new | </span>
        <span className="sc-lineYellow">e</span><span className="sc-lineDim"> edit | </span>
        <span className="sc-lineYellow">s</span><span className="sc-lineDim"> status | </span>
        <span className="sc-lineYellow">/</span><span className="sc-lineDim"> search</span>
      </div>
    </div>
  );
}

function GitMockup() {
  return (
    <div className="sc-mockup sc-mockupGit">
      <div className="sc-mockupHeader">
        <span className="sc-mockupTitle">Git Status</span>
        <span className="sc-lineDim">feature/auth-flow | 3 changed</span>
      </div>
      <div className="sc-mockupBody">
        <div className="sc-mockupSidebar">
          <div className="sc-lineDim" style={{ fontSize: 11, marginBottom: 6 }}>Staged</div>
          <div className="sc-mockupItem sc-mockupItemActive">
            <span className="sc-lineGreen">M</span>
            <span>middleware.go</span>
          </div>
          <div className="sc-mockupItem">
            <span className="sc-lineGreen">A</span>
            <span>jwt.go</span>
          </div>
          <div className="sc-lineDim" style={{ fontSize: 11, marginBottom: 6, marginTop: 12 }}>Unstaged</div>
          <div className="sc-mockupItem">
            <span className="sc-linePink">D</span>
            <span>old_auth.go</span>
          </div>
        </div>
        <div className="sc-mockupMain">
          <div className="sc-mockupDiff">
            <div className="sc-lineDim" style={{ marginBottom: 8 }}>internal/auth/middleware.go</div>
            <div style={{ fontSize: 12, lineHeight: 1.5 }}>
              <div><span className="sc-lineBlue">@@ -42,6 +42,14 @@</span></div>
              <div><span className="sc-lineGreen">+func AuthMiddleware(next http.Handler) {'{'}</span></div>
              <div><span className="sc-lineGreen">+  return http.HandlerFunc(func(w, r) {'{'}</span></div>
              <div><span className="sc-lineGreen">+    token := r.Header.Get("Auth")</span></div>
              <div><span className="sc-lineGreen">+    if !ValidateJWT(token) {'{'}</span></div>
              <div><span className="sc-lineGreen">+      http.Error(w, "Unauth", 401)</span></div>
              <div><span className="sc-lineGreen">+    {'}'}</span></div>
              <div><span className="sc-lineGreen">+  {'}'})</span></div>
              <div><span className="sc-lineGreen">+{'}'}</span></div>
            </div>
          </div>
        </div>
      </div>
      <div className="sc-mockupFooter">
        <span className="sc-lineYellow">a</span><span className="sc-lineDim"> stage | </span>
        <span className="sc-lineYellow">u</span><span className="sc-lineDim"> unstage | </span>
        <span className="sc-lineYellow">c</span><span className="sc-lineDim"> commit | </span>
        <span className="sc-lineYellow">d</span><span className="sc-lineDim"> diff</span>
      </div>
    </div>
  );
}

function FilesMockup() {
  return (
    <div className="sc-mockup sc-mockupFiles">
      <div className="sc-mockupHeader">
        <span className="sc-mockupTitle">File Browser</span>
        <span className="sc-lineDim">sidecar/internal</span>
      </div>
      <div className="sc-mockupBody">
        <div className="sc-mockupSidebar">
          <div className="sc-mockupItem">
            <span className="sc-lineDim">[v]</span>
            <span>internal/</span>
          </div>
          <div className="sc-mockupItem" style={{ paddingLeft: 12 }}>
            <span className="sc-lineDim">[v]</span>
            <span>auth/</span>
          </div>
          <div className="sc-mockupItem sc-mockupItemActive" style={{ paddingLeft: 24 }}>
            <span className="sc-bullet sc-bulletBlue" />
            <span>middleware.go</span>
          </div>
          <div className="sc-mockupItem" style={{ paddingLeft: 24 }}>
            <span className="sc-bullet sc-bulletBlue" />
            <span>jwt.go</span>
          </div>
          <div className="sc-mockupItem" style={{ paddingLeft: 12 }}>
            <span className="sc-lineDim">[&gt;]</span>
            <span>plugins/</span>
          </div>
          <div className="sc-mockupItem" style={{ paddingLeft: 12 }}>
            <span className="sc-lineDim">[&gt;]</span>
            <span>app/</span>
          </div>
        </div>
        <div className="sc-mockupMain">
          <div className="sc-mockupPreview">
            <div className="sc-lineDim" style={{ marginBottom: 8 }}>middleware.go | 156 lines | Go</div>
            <div style={{ fontSize: 12, lineHeight: 1.5 }}>
              <div><span className="sc-lineBlue">package</span> auth</div>
              <div style={{ height: 4 }} />
              <div><span className="sc-lineBlue">import</span> (</div>
              <div>  <span className="sc-lineYellow">"net/http"</span></div>
              <div>  <span className="sc-lineYellow">"strings"</span></div>
              <div>)</div>
              <div style={{ height: 4 }} />
              <div><span className="sc-linePink">// AuthMiddleware validates requests</span></div>
              <div><span className="sc-lineBlue">func</span> AuthMiddleware(next)...</div>
            </div>
          </div>
        </div>
      </div>
      <div className="sc-mockupFooter">
        <span className="sc-lineYellow">enter</span><span className="sc-lineDim"> open | </span>
        <span className="sc-lineYellow">/</span><span className="sc-lineDim"> search | </span>
        <span className="sc-lineYellow">e</span><span className="sc-lineDim"> editor | </span>
        <span className="sc-lineYellow">g</span><span className="sc-lineDim"> goto</span>
      </div>
    </div>
  );
}

function ConversationsMockup() {
  return (
    <div className="sc-mockup sc-mockupConvos">
      <div className="sc-mockupHeader">
        <span className="sc-mockupTitle">Conversations</span>
        <span className="sc-lineDim">12 sessions | Claude Code</span>
      </div>
      <div className="sc-mockupBody">
        <div className="sc-mockupSidebar">
          <div className="sc-mockupItem sc-mockupItemActive">
            <span className="sc-bullet sc-bulletGreen" />
            <div>
              <div>auth-flow-impl</div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>24m ago | 12.4k</div>
            </div>
          </div>
          <div className="sc-mockupItem">
            <span className="sc-bullet sc-bulletBlue" />
            <div>
              <div>fix-rate-limit</div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>2h ago | 8.2k</div>
            </div>
          </div>
          <div className="sc-mockupItem">
            <span className="sc-bullet sc-bulletPink" />
            <div>
              <div>refactor-plugins</div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>1d ago | 24.1k</div>
            </div>
          </div>
        </div>
        <div className="sc-mockupMain">
          <div className="sc-mockupConvo">
            <div style={{ marginBottom: 12 }}>
              <div className="sc-lineBlue" style={{ marginBottom: 4 }}>User</div>
              <div style={{ fontSize: 12 }}>Add JWT authentication to the API endpoints</div>
            </div>
            <div style={{ marginBottom: 12 }}>
              <div className="sc-lineGreen" style={{ marginBottom: 4 }}>Claude</div>
              <div style={{ fontSize: 12 }}>I'll implement JWT authentication for your API.</div>
              <div className="sc-lineDim" style={{ fontSize: 11, marginTop: 4 }}>Let me check the existing auth setup...</div>
            </div>
            <div style={{ fontSize: 11, display: 'grid', gap: 2 }}>
              <div><span className="sc-lineYellow">-&gt;</span> Read middleware.go</div>
              <div><span className="sc-lineYellow">-&gt;</span> Edit jwt.go</div>
              <div><span className="sc-lineYellow">-&gt;</span> Write tests/auth_test.go</div>
            </div>
          </div>
        </div>
      </div>
      <div className="sc-mockupFooter">
        <span className="sc-lineYellow">enter</span><span className="sc-lineDim"> expand | </span>
        <span className="sc-lineYellow">/</span><span className="sc-lineDim"> search | </span>
        <span className="sc-lineYellow">y</span><span className="sc-lineDim"> copy | </span>
        <span className="sc-lineYellow">j/k</span><span className="sc-lineDim"> nav</span>
      </div>
    </div>
  );
}

function WorktreesMockup() {
  return (
    <div className="sc-mockup sc-mockupWorktrees">
      <div className="sc-mockupHeader">
        <span className="sc-mockupTitle">Worktrees</span>
        <span className="sc-lineDim">4 active | 2 with PRs</span>
      </div>
      <div className="sc-mockupBody">
        <div className="sc-mockupSidebar">
          <div className="sc-mockupItem">
            <span className="sc-bullet sc-bulletGreen" />
            <div>
              <div>main</div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>default</div>
            </div>
          </div>
          <div className="sc-mockupItem sc-mockupItemActive">
            <span className="sc-bullet sc-bulletBlue" />
            <div>
              <div>feature/auth</div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>PR #47 | ready</div>
            </div>
          </div>
          <div className="sc-mockupItem">
            <span className="sc-bullet sc-bulletPink" />
            <div>
              <div>fix/memory</div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>PR #52 | review</div>
            </div>
          </div>
          <div className="sc-mockupItem">
            <span className="sc-bullet sc-bulletYellow" />
            <div>
              <div>experiment</div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>no PR</div>
            </div>
          </div>
        </div>
        <div className="sc-mockupMain">
          <div className="sc-mockupWorktree">
            <div className="sc-lineBlue" style={{ fontSize: 14, marginBottom: 8 }}>feature/auth</div>
            <div style={{ display: 'grid', gap: 4, fontSize: 12 }}>
              <div><span className="sc-lineDim">PR:</span> <span className="sc-lineGreen">#47 Add JWT auth</span></div>
              <div><span className="sc-lineDim">Status:</span> <span className="sc-lineGreen">Ready to merge</span></div>
              <div><span className="sc-lineDim">Checks:</span> <span className="sc-lineGreen">All passing</span></div>
              <div><span className="sc-lineDim">Ahead:</span> 3 commits</div>
              <div><span className="sc-lineDim">Behind:</span> 0 commits</div>
            </div>
            <div style={{ marginTop: 12, fontSize: 12 }}>
              <div className="sc-lineDim" style={{ marginBottom: 4 }}>Recent commits</div>
              <div>* Add JWT validation</div>
              <div>* Create auth middleware</div>
              <div>* Initial auth setup</div>
            </div>
          </div>
        </div>
      </div>
      <div className="sc-mockupFooter">
        <span className="sc-lineYellow">enter</span><span className="sc-lineDim"> switch | </span>
        <span className="sc-lineYellow">n</span><span className="sc-lineDim"> new | </span>
        <span className="sc-lineYellow">m</span><span className="sc-lineDim"> merge | </span>
        <span className="sc-lineYellow">d</span><span className="sc-lineDim"> delete</span>
      </div>
    </div>
  );
}

function ComponentSection({ id, title, features, gradient, MockupComponent }) {
  return (
    <div className={`sc-componentSection ${gradient}`} id={id}>
      <div className="sc-componentContent">
        <div className="sc-componentInfo">
          <h3 className="sc-componentTitle">{title}</h3>
          <div className="sc-componentFeatures">
            {features.map((feature, idx) => (
              <div key={idx} className="sc-componentFeature">
                <i className="icon-check sc-featureIcon" />
                <span>{feature}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="sc-componentMockup">
          <MockupComponent />
        </div>
      </div>
    </div>
  );
}

function FeatureListItem({ icon, title, description }) {
  return (
    <div className="sc-featureListItem">
      <div className="sc-featureListIcon">
        <i className={`icon-${icon}`} />
      </div>
      <div className="sc-featureListContent">
        <h4 className="sc-featureListTitle">{title}</h4>
        <p className="sc-featureListDesc">{description}</p>
      </div>
    </div>
  );
}

export default function Home() {
  const { siteConfig } = useDocusaurusContext();
  const [activeTab, setActiveTab] = useState('td');

  const handleTabChange = (tab) => {
    setActiveTab(tab);
  };

  const handleCardClick = (tab) => {
    setActiveTab(tab);
  };

  return (
    <Layout
      title="Terminal UI for AI coding sessions"
      description="Sidecar is a unified terminal interface for monitoring AI coding agent sessions: tasks, git status, files, conversations."
    >
      <header className="sc-hero">
        <div className="container">
          <div className="sc-heroInner">
            <div>
              <h1 className="sc-title">
                Sidecar
                <span className="sc-lineDim"> | </span>
                a terminal cockpit for AI coding
              </h1>

              <p className="sc-subtitle">
                Monitor agent sessions without leaving the terminal: task flow, git diffs, file browsing,
                and conversation history--designed for the "split the terminal and ship" workflow.
              </p>

              <div className="sc-badges" aria-label="Highlights">
                <span className="sc-badge">TUI-first</span>
                <span className="sc-badge">Claude Code | Codex | Gemini | Opencode | Cursor</span>
                <span className="sc-badge">Git + Tasks + Files + Convos</span>
              </div>

              <div className="sc-actions">
                <Link className="sc-btn sc-btnPrimary" to="/docs/intro">
                  Get started <span className="sc-codeInline">curl | bash</span>
                </Link>
                <Link className="sc-btn" to="/docs/intro">
                  Read docs <span className="sc-codeInline">?</span>
                </Link>
                <a className="sc-btn" href={siteConfig.customFields?.githubUrl || 'https://github.com/marcus/sidecar'}>
                  GitHub <span className="sc-codeInline"><i className="icon-external-link" /></span>
                </a>
              </div>

              <div style={{ height: 14 }} />

              <div className="sc-codeBlock sc-installBlock" aria-label="Quick install snippet">
                <div className="sc-installHeader">
                  <span className="sc-lineDim">Quick install</span>
                  <CopyButton text={INSTALL_COMMAND} />
                </div>
                <div className="sc-installCommand">
                  <span className="sc-lineBlue">$ </span>
                  <span>{INSTALL_COMMAND}</span>
                </div>
                <div>
                  <span className="sc-lineBlue">$ </span>
                  <span>sidecar</span>
                </div>
              </div>
            </div>

            <div>
              <Frame activeTab={activeTab} onTabChange={handleTabChange} />
            </div>
          </div>
        </div>
      </header>

      <main className="sc-main">
        {/* Feature Cards */}
        <section className="sc-grid">
          <div className="container">
            <div className="sc-gridInner sc-gridFeatures">
              {/* TD Hero Card - double wide */}
              <FeatureCard
                id="td"
                title="Track work across context windows"
                chip="td"
                isHero={true}
                isHighlighted={activeTab === 'td'}
                onClick={() => handleCardClick('td')}
              >
                The task management plugin designed for AI agents. Track persistent progress, status updates,
                and "what happened while I was gone" across context windows. Create tasks, subtasks, and epics
                that survive session resets.
              </FeatureCard>

              {/* Regular feature cards */}
              <FeatureCard
                id="git"
                title="See what the agent changed"
                chip="git"
                isHighlighted={activeTab === 'git'}
                onClick={() => handleCardClick('git')}
              >
                Split-pane diffs, commit context, and a fast loop for staging/review--without bouncing to an IDE.
              </FeatureCard>

              <FeatureCard
                id="files"
                title="Browse and preview files"
                chip="files"
                isHighlighted={activeTab === 'files'}
                onClick={() => handleCardClick('files')}
              >
                Navigate your codebase with a tree view, preview file contents, and jump to any file instantly.
              </FeatureCard>

              <FeatureCard
                id="conversations"
                title="Resume conversations, not vibes"
                chip="conversations"
                isHighlighted={activeTab === 'conversations'}
                onClick={() => handleCardClick('conversations')}
              >
                Browse sessions, search, and expand message content so you can pick up exactly where the agent left off.
              </FeatureCard>

              <FeatureCard
                id="worktrees"
                title="Manage parallel branches"
                chip="worktrees"
                isHighlighted={activeTab === 'worktrees'}
                onClick={() => handleCardClick('worktrees')}
              >
                View and switch between git worktrees. Track PRs, merge status, and keep multiple features in flight.
              </FeatureCard>
            </div>
          </div>
        </section>

        {/* Component Showcase Sections */}
        <section className="sc-showcase">
          <div className="container">
            <h2 className="sc-showcaseTitle">Component Deep Dive</h2>
            <p className="sc-showcaseSubtitle">Each plugin is designed for the AI-assisted development workflow</p>
          </div>

          <div className="sc-showcaseFullWidth">
            <ComponentSection
              id="showcase-td"
              title="Task Management (td)"
              gradient="sc-gradientGreen"
              MockupComponent={TdMockup}
              features={[
                'Create and track tasks with unique IDs',
                'Hierarchical subtasks and epics',
                'Status workflow: pending, in_progress, blocked, done',
                'Persistent storage survives context resets',
                'Filter by status, search by content',
                'Export tasks as markdown or JSON',
                'Integrate with git commits and PRs',
              ]}
            />

            <ComponentSection
              id="showcase-git"
              title="Git Status & Diff"
              gradient="sc-gradientBlue"
              MockupComponent={GitMockup}
              features={[
                'Real-time status of staged and unstaged changes',
                'Inline diff viewer with syntax highlighting',
                'Stage/unstage files with single keypress',
                'Commit directly from the interface',
                'View commit history and messages',
                'Branch switching and creation',
                'Stash management',
              ]}
            />

            <ComponentSection
              id="showcase-files"
              title="File Browser"
              gradient="sc-gradientPurple"
              MockupComponent={FilesMockup}
              features={[
                'Tree view with expand/collapse',
                'File preview with syntax highlighting',
                'Quick jump with fuzzy search',
                'Show git status indicators on files',
                'Open files in external editor',
                'Navigate to file from other plugins',
                'Respect .gitignore patterns',
              ]}
            />

            <ComponentSection
              id="showcase-conversations"
              title="Conversation Viewer"
              gradient="sc-gradientPink"
              MockupComponent={ConversationsMockup}
              features={[
                'Browse all Claude Code sessions',
                'Search across conversation history',
                'Expand/collapse message content',
                'View tool calls and results',
                'Token usage statistics',
                'Jump to specific conversation turns',
                'Copy message content to clipboard',
              ]}
            />

            <ComponentSection
              id="showcase-worktrees"
              title="Worktree Manager"
              gradient="sc-gradientYellow"
              MockupComponent={WorktreesMockup}
              features={[
                'List all git worktrees',
                'View PR status and checks',
                'Switch between worktrees',
                'Create new worktrees from branches',
                'Delete merged worktrees',
                'See commits ahead/behind main',
                'Merge workflow integration',
              ]}
            />
          </div>
        </section>

        {/* Comprehensive Features Section */}
        <section className="sc-features">
          <div className="container">
            <h2 className="sc-featuresTitle">Features</h2>

            <div className="sc-featuresGrid">
              <FeatureListItem
                icon="terminal"
                title="TUI-First Design"
                description="Built for the terminal from the ground up. No electron, no browser--just fast, keyboard-driven interaction."
              />
              <FeatureListItem
                icon="zap"
                title="Instant Startup"
                description="Launches in milliseconds. No waiting for heavy runtimes or dependency resolution."
              />
              <FeatureListItem
                icon="layout"
                title="Tab-Based Navigation"
                description="Switch between plugins instantly with tab/shift-tab. Each plugin is a focused view of your project."
              />
              <FeatureListItem
                icon="keyboard"
                title="Vim-Style Keybindings"
                description="j/k navigation, /search, and familiar modal interactions. Your muscle memory works here."
              />
              <FeatureListItem
                icon="refresh-cw"
                title="Live Updates"
                description="File changes, git status, and task updates appear in real-time without manual refresh."
              />
              <FeatureListItem
                icon="layers"
                title="Multi-Agent Support"
                description="Works with Claude Code, Codex, Gemini CLI, Opencode, and Cursor's cursor-agent."
              />
              <FeatureListItem
                icon="git-branch"
                title="Git Integration"
                description="Deep integration with git: status, diff, staging, commits, branches, and worktrees."
              />
              <FeatureListItem
                icon="database"
                title="Persistent Storage"
                description="Tasks and preferences persist across sessions. Pick up where you left off."
              />
              <FeatureListItem
                icon="monitor"
                title="tmux Integration"
                description="Designed to run in a tmux pane beside your agent. Attach and detach seamlessly."
              />
              <FeatureListItem
                icon="settings"
                title="Configurable"
                description="Customize keybindings, colors, and behavior. Config files are plain YAML."
              />
              <FeatureListItem
                icon="code"
                title="Open Source"
                description="MIT licensed. Inspect the code, contribute features, or fork for your needs."
              />
              <FeatureListItem
                icon="package"
                title="Single Binary"
                description="No dependencies to install. Download one binary and you're ready to go."
              />
            </div>
          </div>
        </section>

        {/* Supported Agents */}
        <section className="sc-agents">
          <div className="container">
            <h2 className="sc-agentsTitle">Works with your favorite coding agents</h2>
            <p className="sc-agentsSubtitle">
              Sidecar reads session data from multiple AI coding tools, giving you a unified view of agent activity
            </p>

            <div className="sc-agentsGrid">
              <div className="sc-agentCard">
                <div className="sc-agentLogo">
                  <svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="32" height="32" rx="6" fill="#D97706"/>
                    <path d="M16 6L8 10v12l8 4 8-4V10l-8-4zm0 2.2l5.6 2.8L16 13.8l-5.6-2.8L16 8.2zM10 11.8l5 2.5v7.4l-5-2.5v-7.4zm12 0v7.4l-5 2.5v-7.4l5-2.5z" fill="white"/>
                  </svg>
                </div>
                <div className="sc-agentInfo">
                  <h3 className="sc-agentName">Claude Code</h3>
                  <p className="sc-agentDesc">Anthropic's official CLI for Claude</p>
                </div>
              </div>

              <div className="sc-agentCard">
                <div className="sc-agentLogo">
                  <svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="32" height="32" rx="6" fill="#10A37F"/>
                    <circle cx="16" cy="16" r="8" stroke="white" strokeWidth="2" fill="none"/>
                    <circle cx="16" cy="16" r="3" fill="white"/>
                  </svg>
                </div>
                <div className="sc-agentInfo">
                  <h3 className="sc-agentName">Codex</h3>
                  <p className="sc-agentDesc">OpenAI's code generation model</p>
                </div>
              </div>

              <div className="sc-agentCard">
                <div className="sc-agentLogo">
                  <svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="32" height="32" rx="6" fill="#4285F4"/>
                    <path d="M16 8l-6.93 12h13.86L16 8z" fill="#EA4335"/>
                    <path d="M9.07 20L16 8v12H9.07z" fill="#FBBC05"/>
                    <path d="M22.93 20L16 8v12h6.93z" fill="#34A853"/>
                  </svg>
                </div>
                <div className="sc-agentInfo">
                  <h3 className="sc-agentName">Gemini CLI</h3>
                  <p className="sc-agentDesc">Google's multimodal AI assistant</p>
                </div>
              </div>

              <div className="sc-agentCard">
                <div className="sc-agentLogo">
                  <svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="32" height="32" rx="6" fill="#6366F1"/>
                    <path d="M10 10h12v12H10V10z" stroke="white" strokeWidth="2" fill="none"/>
                    <path d="M14 14h4v4h-4v-4z" fill="white"/>
                  </svg>
                </div>
                <div className="sc-agentInfo">
                  <h3 className="sc-agentName">Opencode</h3>
                  <p className="sc-agentDesc">Terminal-first AI coding assistant</p>
                </div>
              </div>

              <div className="sc-agentCard">
                <div className="sc-agentLogo">
                  <svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="32" height="32" rx="6" fill="#171717"/>
                    <path d="M8 16a8 8 0 1 1 16 0" stroke="#F7B500" strokeWidth="2.5" strokeLinecap="round"/>
                    <circle cx="16" cy="16" r="3" fill="#F7B500"/>
                    <path d="M16 19v5" stroke="#F7B500" strokeWidth="2" strokeLinecap="round"/>
                  </svg>
                </div>
                <div className="sc-agentInfo">
                  <h3 className="sc-agentName">Cursor</h3>
                  <p className="sc-agentDesc">AI-first code editor (cursor-agent)</p>
                </div>
              </div>
            </div>

            <p className="sc-agentsNote">
              Each agent stores session data in its own format. Sidecar normalizes and displays them in a unified interface.
            </p>
          </div>
        </section>
      </main>
    </Layout>
  );
}
