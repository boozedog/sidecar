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
      <p className="sc-sectionTitle">All Sessions <span className="sc-lineDim">chronological</span></p>
      <div className="sc-list">
        <div className="sc-item sc-itemActive">
          <span className="sc-bullet sc-bulletGreen" />
          <span>auth-flow | <span className="sc-lineYellow">Claude</span> | 24m</span>
        </div>
        <div className="sc-item">
          <span className="sc-bullet sc-bulletBlue" />
          <span>rate-limit | <span className="sc-linePink">Cursor</span> | 2h</span>
        </div>
        <div className="sc-item">
          <span className="sc-bullet sc-bulletPink" />
          <span>refactor | <span className="sc-lineBlue">Gemini</span> | 1d</span>
        </div>
      </div>
      <div style={{ height: 12 }} />
      <div className="sc-codeBlock">
        <div className="sc-lineDim">auth-flow | <span className="sc-lineYellow">Claude Code</span></div>
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
      <p className="sc-sectionTitle">Worktrees <span className="sc-lineDim">zero commands</span></p>
      <div className="sc-list">
        <div className="sc-item">
          <span className="sc-bullet sc-bulletGreen" />
          main
        </div>
        <div className="sc-item sc-itemActive">
          <span className="sc-bullet sc-bulletBlue" />
          feature/auth <span className="sc-lineDim">PR #47</span>
        </div>
        <div className="sc-item">
          <span className="sc-bullet sc-bulletPink" />
          fix/memory <span className="sc-lineDim">PR #52</span>
        </div>
      </div>
      <div style={{ height: 12 }} />
      <div className="sc-codeBlock" role="img" aria-label="Sidecar output pane preview">
        <div className="sc-lineDim">feature/auth | <span className="sc-lineGreen">Ready to merge</span></div>
        <div style={{ height: 6 }} />
        <div><span className="sc-lineBlue">Task:</span> td-a1b2c3 <span className="sc-lineDim">from td</span></div>
        <div><span className="sc-lineYellow">Prompts:</span> 3 configured</div>
        <div style={{ height: 6 }} />
        <div className="sc-lineDim">Actions:</div>
        <div>  <span className="sc-lineGreen">[n]</span> New worktree + agent</div>
        <div>  <span className="sc-lineGreen">[s]</span> Send task from td</div>
        <div>  <span className="sc-lineGreen">[p]</span> Run prompt sequence</div>
        <div style={{ height: 6 }} />
        <div className="sc-lineDim">* 3 commits ahead | checks passing</div>
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
        <span className="sc-lineDim">18 sessions | all agents</span>
      </div>
      <div className="sc-mockupBody">
        <div className="sc-mockupSidebar">
          <div className="sc-mockupItem sc-mockupItemActive">
            <span className="sc-bullet sc-bulletGreen" />
            <div>
              <div>auth-flow <span className="sc-lineYellow" style={{ fontSize: 10 }}>Claude</span></div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>24m ago | 12.4k</div>
            </div>
          </div>
          <div className="sc-mockupItem">
            <span className="sc-bullet sc-bulletBlue" />
            <div>
              <div>rate-limit <span className="sc-linePink" style={{ fontSize: 10 }}>Cursor</span></div>
              <div className="sc-lineDim" style={{ fontSize: 11 }}>2h ago | 8.2k</div>
            </div>
          </div>
          <div className="sc-mockupItem">
            <span className="sc-bullet sc-bulletPink" />
            <div>
              <div>refactor <span className="sc-lineBlue" style={{ fontSize: 10 }}>Gemini</span></div>
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
        <span className="sc-lineDim">zero commands | auto everything</span>
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
              <div className="sc-lineDim" style={{ fontSize: 11 }}>td-g7h8i9</div>
            </div>
          </div>
        </div>
        <div className="sc-mockupMain">
          <div className="sc-mockupWorktree">
            <div className="sc-lineBlue" style={{ fontSize: 14, marginBottom: 8 }}>feature/auth</div>
            <div style={{ display: 'grid', gap: 4, fontSize: 12 }}>
              <div><span className="sc-lineDim">PR:</span> <span className="sc-lineGreen">#47 Add JWT auth</span></div>
              <div><span className="sc-lineDim">Task:</span> <span className="sc-lineYellow">td-a1b2c3</span> <span className="sc-lineDim">from td</span></div>
              <div><span className="sc-lineDim">Status:</span> <span className="sc-lineGreen">Ready to merge</span></div>
            </div>
            <div style={{ marginTop: 12, fontSize: 12 }}>
              <div className="sc-lineDim" style={{ marginBottom: 4 }}>Quick actions</div>
              <div><span className="sc-lineGreen">[n]</span> New worktree + start agent</div>
              <div><span className="sc-lineGreen">[s]</span> Send task from td</div>
              <div><span className="sc-lineGreen">[p]</span> Run prompt sequence</div>
              <div><span className="sc-lineGreen">[m]</span> Merge & cleanup</div>
            </div>
          </div>
        </div>
      </div>
      <div className="sc-mockupFooter">
        <span className="sc-lineYellow">n</span><span className="sc-lineDim"> new | </span>
        <span className="sc-lineYellow">s</span><span className="sc-lineDim"> send task | </span>
        <span className="sc-lineYellow">p</span><span className="sc-lineDim"> prompts | </span>
        <span className="sc-lineYellow">m</span><span className="sc-lineDim"> merge</span>
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

function FeatureListItem({ icon, title, description, color }) {
  return (
    <div className="sc-featureListItem">
      <div className="sc-featureListHeader">
        <h4 className={`sc-featureListTitle ${color ? `sc-featureColor-${color}` : ''}`}>{title}</h4>
        <div className="sc-featureListIcon">
          <i className={`icon-${icon}`} />
        </div>
      </div>
      <p className="sc-featureListDesc">{description}</p>
    </div>
  );
}

function WorkflowSection() {
  return (
    <section className="sc-workflow">
      <div className="container">
        <h2 className="sc-showcaseTitle" style={{ textAlign: 'center', marginBottom: '48px' }}>The Workflow</h2>
        
        <div className="sc-workflowGrid">
          
          <div className="sc-workflowStep sc-step-green">
            <div className="sc-workflowIconBox">
              <i className="icon-list sc-workflowIcon" />
            </div>
            <div>
              <h3 className="sc-workflowTitle">1. Plan</h3>
              <p className="sc-workflowDesc">
                Create tasks in <a href="https://github.com/marcus/td" className="sc-inlineLink">td</a> to give agents clear objectives and context.
              </p>
            </div>
          </div>

          <div className="sc-workflowStep sc-step-blue">
            <div className="sc-workflowIconBox">
              <i className="icon-terminal sc-workflowIcon" />
            </div>
            <div>
              <h3 className="sc-workflowTitle">2. Act</h3>
              <p className="sc-workflowDesc">
                Your agent (Claude, Cursor, etc.) reads the task and writes code.
              </p>
            </div>
          </div>

          <div className="sc-workflowStep sc-step-purple">
            <div className="sc-workflowIconBox">
              <i className="icon-eye sc-workflowIcon" />
            </div>
            <div>
              <h3 className="sc-workflowTitle">3. Monitor</h3>
              <p className="sc-workflowDesc">
                Watch the agent's progress, diffs, and logs in Sidecar's TUI.
              </p>
            </div>
          </div>

          <div className="sc-workflowStep sc-step-pink">
            <div className="sc-workflowIconBox">
              <i className="icon-check sc-workflowIcon" />
            </div>
            <div>
              <h3 className="sc-workflowTitle">4. Review</h3>
              <p className="sc-workflowDesc">
                Verify the changes, commit, and mark the task as done.
              </p>
            </div>
          </div>

        </div>
      </div>
    </section>
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
      title="You might never open your editor again"
      description="AI agents write your code. Sidecar keeps you in the terminal for everything else: planning tasks, reviewing diffs, staging commits, managing worktrees."
    >
      <header className="sc-hero">
        <div className="container">
          <div className="sc-heroInner">
            <div>
              <h1 className="sc-title">
                <span className="sc-titleBrand">Sidecar</span>
                <span className="sc-titleTagline">You might never open your editor again.</span>
              </h1>

              <p className="sc-subtitle">
                AI agents write your code. Sidecar keeps you in the terminal for everything else:
                planning tasks with <a href="https://github.com/marcus/td" className="sc-inlineLink">td</a>, reviewing diffs, staging commits, managing worktrees. The whole development loop, one interface.
              </p>

              <div style={{ height: 32 }} />

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

              <div style={{ height: 28 }} />

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
            <p className="sc-gridHint">Click a card to see it in action above</p>
            <div className="sc-gridInner sc-gridFeatures">
              {/* TD Hero Card - double wide */}
              <FeatureCard
                id="td"
                title={<>Plan with <a href="https://github.com/marcus/td" className="sc-inlineLink">td</a></>}
                chip="td"
                isHero={true}
                isHighlighted={activeTab === 'td'}
                onClick={() => handleCardClick('td')}
              >
                Give agents structured work so they can operate autonomously for longer. Tasks persist across
                context windows, keeping agents on track with clear objectives. Built-in review workflow
                lets you verify work before moving to the next task.
              </FeatureCard>

              {/* Regular feature cards */}
              <FeatureCard
                id="git"
                title="See what the agent changed"
                chip="git"
                isHighlighted={activeTab === 'git'}
                onClick={() => handleCardClick('git')}
              >
                Split-pane diffs, commit context, fast staging—all without bouncing to an IDE.
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
                title="One timeline, all agents"
                chip="conversations"
                isHighlighted={activeTab === 'conversations'}
                onClick={() => handleCardClick('conversations')}
              >
                All your agents in one timeline—Claude, Cursor, Gemini, and more. Search across sessions, pick up where any agent left off.
              </FeatureCard>

              <FeatureCard
                id="worktrees"
                title="Zero-command worktree workflow"
                chip="worktrees"
                isHighlighted={activeTab === 'worktrees'}
                onClick={() => handleCardClick('worktrees')}
              >
                Create worktrees, pass tasks from td, kick off with configured prompts—no git commands needed. Everything is automatic.
              </FeatureCard>
            </div>
          </div>
        </section>

        <WorkflowSection />

        {/* Component Showcase Sections */}
        <section className="sc-showcase">
          <div className="container">
            <h2 className="sc-showcaseTitle">The Plugins</h2>
            <p className="sc-showcaseSubtitle">Each one built for AI-assisted development</p>
          </div>

          <div className="sc-showcaseFullWidth">
            <ComponentSection
              id="showcase-td"
              title={<>Plan with <a href="https://github.com/marcus/td" className="sc-inlineLink">td</a></>}
              gradient="sc-gradientGreen"
              MockupComponent={TdMockup}
              features={[
                'Structured work keeps agents focused and autonomous',
                'Tasks persist across context window resets',
                'Built-in review workflow: verify before moving on',
                'Hierarchical subtasks break down complex work',
                'Status tracking: pending, in_progress, blocked, done',
                'Epics group related tasks for larger features',
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
              title="Unified Conversation Timeline"
              gradient="sc-gradientPink"
              MockupComponent={ConversationsMockup}
              features={[
                'Chronological view across all coding agents',
                'Claude, Cursor, Gemini, Codex, Opencode in one list',
                'Search across all adapters at once',
                'Filter by agent, date, or content',
                'Expand messages and view tool calls',
                'Token usage and session duration stats',
                'Copy and export conversation content',
              ]}
            />

            <ComponentSection
              id="showcase-worktrees"
              title="Zero-Command Worktree Workflow"
              gradient="sc-gradientYellow"
              MockupComponent={WorktreesMockup}
              features={[
                'No git commands needed--everything is automatic',
                'Pass tasks directly from td to new worktrees',
                'Configure prompt sequences to kick off agents',
                'Create, switch, merge, delete with single keys',
                'PR status and CI checks at a glance',
                'Auto-cleanup after merge',
                'Linked task tracking across worktrees',
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
                icon="feather"
                title="Zero Config Setup"
                color="green"
                description="Just add a couple lines to your AGENTS.md file. No hooks, no agent modifications. Easy to add, easy to remove."
              />
              <FeatureListItem
                icon="zap"
                title="Instant Startup"
                color="yellow"
                description="Launches in milliseconds. No waiting for heavy runtimes or dependency resolution."
              />
              <FeatureListItem
                icon="columns-2"
                title="Tab-Based Navigation"
                color="blue"
                description="Switch between plugins instantly with tab/shift-tab. Each plugin is a focused view of your project."
              />
              <FeatureListItem
                icon="mouse-pointer"
                title="Full Mouse Support"
                color="purple"
                description="Click, scroll, and navigate with your mouse. Almost every element in the TUI responds to mouse interaction."
              />
              <FeatureListItem
                icon="refresh-cw"
                title="Auto-Update"
                color="pink"
                description="Sidecar checks for updates automatically and can update itself in place. Always stay on the latest version."
              />
              <FeatureListItem
                icon="layers"
                title="Multi-Agent Support"
                color="orange"
                description="Works with Claude Code, Codex, Gemini CLI, Opencode, and Cursor's cursor-agent."
              />
              <FeatureListItem
                icon="git-branch"
                title="Git Integration"
                color="blue"
                description="Deep integration with git: status, diff, staging, commits, branches, and worktrees."
              />
              <FeatureListItem
                icon="palette"
                title="Custom Themes"
                color="green"
                description="Ship with multiple themes or create your own. Customize colors to match your terminal aesthetic."
              />
              <FeatureListItem
                icon="monitor"
                title="tmux Integration"
                color="yellow"
                description="Designed to run in a tmux pane beside your agent. Attach and detach seamlessly."
              />
              <FeatureListItem
                icon="keyboard"
                title="Vim-Style Keybindings"
                color="purple"
                description="j/k navigation, /search, and familiar modal interactions. Your muscle memory works here."
              />
              <FeatureListItem
                icon="code"
                title="Open Source"
                color="pink"
                description="MIT licensed. Inspect the code, contribute features, or fork for your needs."
              />
              <FeatureListItem
                icon="package"
                title="Single Binary"
                color="orange"
                description="No dependencies to install. Download one binary and you're ready to go."
              />
            </div>
          </div>
        </section>

        {/* Supported Agents */}
        <section className="sc-agents">
          <div className="container">
            <h2 className="sc-agentsTitle">Supported Agents</h2>
            <p className="sc-agentsSubtitle">
              Sidecar reads session data from multiple AI coding tools, giving you a unified view of agent activity
            </p>

            <div className="sc-agentsGrid">
              <div className="sc-agentCard">
                <div className="sc-agentLogo">
                  <svg viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="32" height="32" rx="6" fill="#D97706" />
                    <path d="M16 6L8 10v12l8 4 8-4V10l-8-4zm0 2.2l5.6 2.8L16 13.8l-5.6-2.8L16 8.2zM10 11.8l5 2.5v7.4l-5-2.5v-7.4zm12 0v7.4l-5 2.5v-7.4l5-2.5z" fill="white" />
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
                    <rect width="32" height="32" rx="6" fill="#10A37F" />
                    <circle cx="16" cy="16" r="8" stroke="white" strokeWidth="2" fill="none" />
                    <circle cx="16" cy="16" r="3" fill="white" />
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
                    <rect width="32" height="32" rx="6" fill="#4285F4" />
                    <path d="M16 8l-6.93 12h13.86L16 8z" fill="#EA4335" />
                    <path d="M9.07 20L16 8v12H9.07z" fill="#FBBC05" />
                    <path d="M22.93 20L16 8v12h6.93z" fill="#34A853" />
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
                    <rect width="32" height="32" rx="6" fill="#6366F1" />
                    <path d="M10 10h12v12H10V10z" stroke="white" strokeWidth="2" fill="none" />
                    <path d="M14 14h4v4h-4v-4z" fill="white" />
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
                    <rect width="32" height="32" rx="6" fill="#171717" />
                    <path d="M8 16a8 8 0 1 1 16 0" stroke="#F7B500" strokeWidth="2.5" strokeLinecap="round" />
                    <circle cx="16" cy="16" r="3" fill="#F7B500" />
                    <path d="M16 19v5" stroke="#F7B500" strokeWidth="2" strokeLinecap="round" />
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

        {/* Bottom CTA */}
        <section className="sc-bottomCta">
          <div className="container">
            <h2 className="sc-bottomCtaTitle">Get started in seconds</h2>
            <div className="sc-codeBlock sc-installBlock sc-bottomInstall">
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
            <div className="sc-bottomCtaLinks">
              <Link className="sc-btn sc-btnPrimary" to="/docs/intro">
                Read the docs
              </Link>
              <a className="sc-btn" href="https://github.com/marcus/sidecar">
                View on GitHub
              </a>
            </div>
          </div>
        </section>
      </main>
    </Layout>
  );
}
