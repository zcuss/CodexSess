<script>
  let {
    busy,
    apiMode,
    onSetAPIMode,
    showAccountEmail,
    onToggleShowAccountEmail,
    directAPIStrategy,
    codingTemplateHome,
    codingTemplateBusy,
    usageAlertThreshold,
    usageAlertThresholdInput,
    usageAutoSwitchThreshold,
    usageAutoSwitchThresholdInput,
    usageSchedulerIntervalMinutes,
    usageSchedulerIntervalMinutesInput,
    usageSoundEnabled,
    onSetDirectAPIStrategy,
    onInitializeCodingTemplateHome,
    onResyncCodingTemplateHome,
    onRefreshCodingTemplateHome,
    onSetUsageAlertThresholdInput,
    onCommitUsageAlertThresholdInput,
    onSetUsageAutoSwitchThresholdInput,
    onCommitUsageAutoSwitchThresholdInput,
    onSetUsageSchedulerIntervalInput,
    onCommitUsageSchedulerIntervalInput,
    onNudgeUsageAlertThreshold,
    onNudgeUsageAutoSwitchThreshold,
    onNudgeUsageSchedulerInterval,
    onToggleUsageSoundEnabled,
    channels,
    onUpdateChannelField,
    onSaveChannelIntegrations,
    channelPairingPending,
    channelPairingLinks,
    channelPairingSessionID,
    onSetChannelPairingSessionID,
    onRefreshChannelPairing,
    onApproveChannelPairing,
    onRevokeChannelPairing,
    selfHeal,
    selfHealGitHubTokenInput,
    selfHealClearGitHubToken,
    selfHealGitRemoteURL,
    selfHealTestPushResult,
    selfHealSyncResult,
    selfHealForcePushResult,
    onSetSelfHealGitHubTokenInput,
    onSetSelfHealClearGitHubToken,
    onSetSelfHealGitRemoteURL,
    onUpdateSelfHealField,
    onSaveSelfHealSettings,
    onSaveSelfHealGitRemote,
    onSyncSelfHealRemote,
    onForcePushSelfHealRemote,
    onTestSelfHealPush
  } = $props();

  function nudgeAlert(delta) {
    onNudgeUsageAlertThreshold(delta);
  }

  function nudgeAutoSwitch(delta) {
    onNudgeUsageAutoSwitchThreshold(delta);
  }

  function nudgeSchedulerInterval(delta) {
    onNudgeUsageSchedulerInterval(delta);
  }
</script>

<section class="panel">
  <div class="panel-header">
    <h2>Settings</h2>
  </div>

  <div class="settings-list">
    <section class="setting-category">
      <h3 class="setting-category-title">API Mode</h3>
      <div class="setting-row">
        <p class="setting-title">Proxy Execution Mode</p>
        <div class="api-mode-switch" role="group" aria-label="API mode switch">
          <button
            type="button"
            class="btn btn-secondary api-mode-btn {apiMode === 'codex_cli' ? 'is-active' : ''}"
            onclick={() => onSetAPIMode('codex_cli')}
            disabled={busy || apiMode === 'codex_cli'}
            aria-pressed={apiMode === 'codex_cli'}
          >
            Codex CLI
          </button>
          <button
            type="button"
            class="btn btn-secondary api-mode-btn {apiMode === 'direct_api' ? 'is-active' : ''}"
            onclick={() => onSetAPIMode('direct_api')}
            disabled={busy || apiMode === 'direct_api'}
            aria-pressed={apiMode === 'direct_api'}
          >
            Direct API
          </button>
        </div>
        <p class="setting-title">
          {#if apiMode === 'direct_api'}
            /v1 endpoints call ChatGPT backend API directly.
          {:else}
            /v1 endpoints call local codex CLI execution pipeline.
          {/if}
        </p>
      </div>
      {#if apiMode === 'direct_api'}
      <div class="setting-row">
        <p class="setting-title">Direct API Account Strategy</p>
        <div class="api-mode-switch" role="group" aria-label="direct api strategy switch">
          <button
            type="button"
            class="btn btn-secondary api-mode-btn {directAPIStrategy === 'round_robin' ? 'is-active' : ''}"
            onclick={() => onSetDirectAPIStrategy('round_robin')}
            disabled={busy || directAPIStrategy === 'round_robin'}
            aria-pressed={directAPIStrategy === 'round_robin'}
          >
            Round Robin
          </button>
          <button
            type="button"
            class="btn btn-secondary api-mode-btn {directAPIStrategy === 'load_balance' ? 'is-active' : ''}"
            onclick={() => onSetDirectAPIStrategy('load_balance')}
            disabled={busy || directAPIStrategy === 'load_balance'}
            aria-pressed={directAPIStrategy === 'load_balance'}
          >
            Load Balance
          </button>
        </div>
        <p class="setting-title">
          {#if directAPIStrategy === 'load_balance'}
            Select account by highest fresh remaining usage.
          {:else}
            Rotate account every request to distribute load.
          {/if}
        </p>
      </div>
      {/if}
    </section>

    <section class="setting-category">
      <h3 class="setting-category-title">Account Display</h3>
      <div class="setting-row">
        <p class="setting-title">Managed Account Information</p>
        <div class="setting-actions-grid with-three">
          <input value={showAccountEmail ? 'Email is visible in Managed Accounts' : 'Email is hidden, showing account ID'} readonly disabled />
          <button class="btn btn-secondary" onclick={onToggleShowAccountEmail}>
            {#if showAccountEmail}Hide Information{:else}Show Information{/if}
          </button>
        </div>
      </div>
    </section>

    <section class="setting-category">
      <h3 class="setting-category-title">Template Home</h3>
      <div class="setting-row">
        <p class="setting-title">Base Codex Home</p>
        <div class="setting-actions-grid with-three">
          <input value={codingTemplateHome?.root_path || 'Not loaded'} readonly disabled />
          <button class="btn btn-secondary" onclick={onRefreshCodingTemplateHome} disabled={busy || codingTemplateBusy}>
            Refresh Status
          </button>
          <button class="btn btn-secondary" onclick={onInitializeCodingTemplateHome} disabled={busy || codingTemplateBusy}>
            Initialize
          </button>
          <button class="btn btn-secondary" onclick={onResyncCodingTemplateHome} disabled={busy || codingTemplateBusy}>
            Resync
          </button>
        </div>
        <p class="setting-title">
          {#if codingTemplateHome?.ready}
            Template is ready with baseline MCP servers.
          {:else if codingTemplateHome}
            Template is missing baseline fields: {(codingTemplateHome.missing_baseline_fields || []).join(', ') || 'unknown'}
          {:else}
            Template status has not been loaded yet.
          {/if}
        </p>
        <p class="setting-title">
          {codingTemplateHome?.config_path ? `Config: ${codingTemplateHome.config_path}` : ''}
        </p>
        <p class="setting-title">
          {codingTemplateHome?.runtime_home_count != null ? `Runtime homes: ${codingTemplateHome.runtime_home_count}` : ''}
        </p>
      </div>
      <div class="setting-row">
        <p class="setting-title">Seeded MCP</p>
        <p class="setting-title">
          {(codingTemplateHome?.enabled_mcp_servers || []).join(', ') || 'None'}
        </p>
        <p class="setting-title">
          Disabled: {(codingTemplateHome?.disabled_mcp_servers || []).join(', ') || 'None'}
        </p>
      </div>
    </section>

    <section class="setting-category">
      <h3 class="setting-category-title">Channel Integrations</h3>
      <div class="setting-row">
        <p class="setting-title">Telegram Bot</p>
        <div class="setting-actions-grid with-three">
          <input value={channels?.telegram?.bot_token || ''} placeholder="Bot token" oninput={(event) => onUpdateChannelField('telegram.bot_token', event.currentTarget.value)} />
          <input value={channels?.telegram?.secret_token || ''} placeholder="Secret token (optional)" oninput={(event) => onUpdateChannelField('telegram.secret_token', event.currentTarget.value)} />
          <input value={channels?.telegram?.model || ''} placeholder="Model" oninput={(event) => onUpdateChannelField('telegram.model', event.currentTarget.value)} />
        </div>
        <div class="inline-actions">
          <label><input type="checkbox" checked={Boolean(channels?.telegram?.enabled)} onchange={(event) => onUpdateChannelField('telegram.enabled', event.currentTarget.checked)} /> Enabled</label>
          <label><input type="checkbox" checked={channels?.telegram?.reply_enabled !== false} onchange={(event) => onUpdateChannelField('telegram.reply_enabled', event.currentTarget.checked)} /> Auto Reply</label>
        </div>
        <p class="setting-title">Inbound mode: <code>polling</code> (no webhook/public IP required)</p>
      </div>

      <div class="setting-row">
        <p class="setting-title">Discord Webhook Bridge</p>
        <div class="setting-actions-grid with-three">
          <input value={channels?.discord?.inbound_secret || ''} placeholder="Inbound secret" oninput={(event) => onUpdateChannelField('discord.inbound_secret', event.currentTarget.value)} />
          <input value={channels?.discord?.webhook_url || ''} placeholder="Discord webhook URL" oninput={(event) => onUpdateChannelField('discord.webhook_url', event.currentTarget.value)} />
          <input value={channels?.discord?.model || ''} placeholder="Model" oninput={(event) => onUpdateChannelField('discord.model', event.currentTarget.value)} />
        </div>
        <div class="inline-actions">
          <label><input type="checkbox" checked={Boolean(channels?.discord?.enabled)} onchange={(event) => onUpdateChannelField('discord.enabled', event.currentTarget.checked)} /> Enabled</label>
          <label><input type="checkbox" checked={channels?.discord?.reply_enabled !== false} onchange={(event) => onUpdateChannelField('discord.reply_enabled', event.currentTarget.checked)} /> Auto Reply</label>
        </div>
        <p class="setting-title">Inbound endpoint menerima JSON bridge: <code>/api/channels/discord/webhook</code></p>
      </div>

      <div class="setting-row">
        <p class="setting-title">WhatsApp Cloud API</p>
        <div class="setting-actions-grid with-three">
          <input value={channels?.whatsapp?.verify_token || ''} placeholder="Verify token" oninput={(event) => onUpdateChannelField('whatsapp.verify_token', event.currentTarget.value)} />
          <input value={channels?.whatsapp?.access_token || ''} placeholder="Access token" oninput={(event) => onUpdateChannelField('whatsapp.access_token', event.currentTarget.value)} />
          <input value={channels?.whatsapp?.phone_number_id || ''} placeholder="Phone number ID" oninput={(event) => onUpdateChannelField('whatsapp.phone_number_id', event.currentTarget.value)} />
          <input value={channels?.whatsapp?.model || ''} placeholder="Model" oninput={(event) => onUpdateChannelField('whatsapp.model', event.currentTarget.value)} />
        </div>
        <div class="inline-actions">
          <label><input type="checkbox" checked={Boolean(channels?.whatsapp?.enabled)} onchange={(event) => onUpdateChannelField('whatsapp.enabled', event.currentTarget.checked)} /> Enabled</label>
          <label><input type="checkbox" checked={channels?.whatsapp?.reply_enabled !== false} onchange={(event) => onUpdateChannelField('whatsapp.reply_enabled', event.currentTarget.checked)} /> Auto Reply</label>
        </div>
        <p class="setting-title">Verify (GET) + webhook (POST): <code>/api/channels/whatsapp/webhook</code></p>
      </div>

      <div class="setting-row">
        <button class="btn btn-primary" onclick={onSaveChannelIntegrations} disabled={busy}>Save Channel Integrations</button>
      </div>
    </section>

    <section class="setting-category">
      <h3 class="setting-category-title">Channel Pairing</h3>
      <div class="setting-row">
        <p class="setting-title">Approve pending pair request from channel command <code>/pair</code></p>
        <div class="setting-actions-grid with-three">
          <input
            value={channelPairingSessionID || ''}
            placeholder="Optional existing session_id (leave empty = create new)"
            oninput={(event) => onSetChannelPairingSessionID(event.currentTarget.value)}
          />
          <button class="btn btn-secondary" onclick={onRefreshChannelPairing} disabled={busy}>Refresh Pairing</button>
        </div>
      </div>
      <div class="setting-row">
        <p class="setting-title">Pending Requests</p>
        {#if Array.isArray(channelPairingPending) && channelPairingPending.length > 0}
          <div class="usage-list">
            {#each channelPairingPending as item}
              <div class="usage-item">
                <div class="usage-top">
                  <p>{item.channel}:{item.user_id}</p>
                  <p>Code {item.code}</p>
                </div>
                <p class="usage-reset">Expires: {item.expires_at || '-'}</p>
                <div class="inline-actions">
                  <button class="btn btn-small btn-primary" onclick={() => onApproveChannelPairing(item)} disabled={busy}>Approve</button>
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="empty-state compact">No pending requests.</div>
        {/if}
      </div>
      <div class="setting-row">
        <p class="setting-title">Active Links</p>
        {#if Array.isArray(channelPairingLinks) && channelPairingLinks.length > 0}
          <div class="usage-list">
            {#each channelPairingLinks as item}
              <div class="usage-item">
                <div class="usage-top">
                  <p>{item.channel}:{item.user_id}</p>
                  <p><a href={`/chat?id=${encodeURIComponent(item.session_id || '')}`}>chat?id={item.session_id}</a></p>
                </div>
                <p class="usage-reset">Paired: {item.paired_at || '-'} | Model: {item.model_override || 'channel default'}</p>
                <div class="inline-actions">
                  <button class="btn btn-small btn-danger" onclick={() => onRevokeChannelPairing(item)} disabled={busy}>Revoke</button>
                </div>
              </div>
            {/each}
          </div>
        {:else}
          <div class="empty-state compact">No active links.</div>
        {/if}
      </div>
    </section>

    <section class="setting-category">
      <h3 class="setting-category-title">Self-Heal</h3>
      <div class="setting-row">
        <p class="setting-title">Automatic self-improvement and GitHub push</p>
        <div class="inline-actions">
          <label><input type="checkbox" checked={Boolean(selfHeal?.enabled)} onchange={(event) => onUpdateSelfHealField('enabled', event.currentTarget.checked)} /> Enabled</label>
          <label><input type="checkbox" checked={selfHeal?.on_error !== false} onchange={(event) => onUpdateSelfHealField('on_error', event.currentTarget.checked)} /> Trigger on error</label>
          <label><input type="checkbox" checked={selfHeal?.auto_push !== false} onchange={(event) => onUpdateSelfHealField('auto_push', event.currentTarget.checked)} /> Auto push</label>
        </div>
      </div>
      <div class="setting-row">
        <p class="setting-title">Fix Command</p>
        <div class="setting-actions-grid with-three">
          <input value={selfHeal?.command || ''} placeholder="PowerShell command to run fix pipeline" oninput={(event) => onUpdateSelfHealField('command', event.currentTarget.value)} />
          <input value={selfHeal?.git_remote || 'origin'} placeholder="Git remote" oninput={(event) => onUpdateSelfHealField('git_remote', event.currentTarget.value)} />
          <input value={selfHeal?.git_branch || 'main'} placeholder="Git branch" oninput={(event) => onUpdateSelfHealField('git_branch', event.currentTarget.value)} />
          <input value={selfHealGitRemoteURL || ''} placeholder="Git remote URL (https://github.com/user/repo.git)" oninput={(event) => onSetSelfHealGitRemoteURL(event.currentTarget.value)} />
          <input value={selfHeal?.commit_prefix || 'self-heal'} placeholder="Commit prefix" oninput={(event) => onUpdateSelfHealField('commit_prefix', event.currentTarget.value)} />
          <input type="password" value={selfHealGitHubTokenInput || ''} placeholder="GitHub token (PAT) - leave blank to keep current" oninput={(event) => onSetSelfHealGitHubTokenInput(event.currentTarget.value)} />
        </div>
        <div class="inline-actions">
          <label><input type="checkbox" checked={Boolean(selfHealClearGitHubToken)} onchange={(event) => onSetSelfHealClearGitHubToken(event.currentTarget.checked)} /> Clear saved token</label>
        </div>
        <p class="setting-title">GitHub token status: {selfHeal?.has_github_token ? 'saved' : 'not set'}</p>
        <p class="setting-title">Jika ada error, engine akan jalankan command, cek build, lalu commit + push ke branch target.</p>
      </div>
      <div class="setting-row">
        <button class="btn btn-primary" onclick={onSaveSelfHealSettings} disabled={busy}>Save Self-Heal</button>
        <button class="btn btn-secondary" onclick={onSaveSelfHealGitRemote} disabled={busy}>Save Git Remote</button>
        <button class="btn btn-secondary" onclick={onSyncSelfHealRemote} disabled={busy}>Sync Remote</button>
        <button class="btn btn-secondary" onclick={onTestSelfHealPush} disabled={busy}>Test Push</button>
        <button class="btn btn-danger" onclick={onForcePushSelfHealRemote} disabled={busy}>Force Push Main</button>
      </div>
      <div class="setting-row">
        <p class="setting-title">Sync Result</p>
        <pre>{selfHealSyncResult || '-'}</pre>
      </div>
      <div class="setting-row">
        <p class="setting-title">Test Push Result</p>
        <pre>{selfHealTestPushResult || '-'}</pre>
      </div>
      <div class="setting-row">
        <p class="setting-title">Force Push Result</p>
        <pre>{selfHealForcePushResult || '-'}</pre>
      </div>
    </section>

    <section class="setting-category">
      <h3 class="setting-category-title">Usage Automation</h3>
      <div class="setting-row">
        <p class="setting-title">Background Usage Scheduler</p>
        <p class="setting-title">Backend job ini refresh usage semua akun non-revoked di database setiap siklus sesuai interval settings. Job ini tidak menjalankan auto-switch akun aktif.</p>
      </div>

      <div class="setting-row">
        <p class="setting-title">Background Usage Refresh Interval (minutes)</p>
        <div class="slider-wrap">
        <div class="slider-head">
          <span class="setting-title">Run background refresh every</span>
          <div class="inline-actions">
            <button type="button" class="btn btn-small btn-secondary" onclick={() => nudgeSchedulerInterval(-1)}>-</button>
            <span class="slider-value">{usageSchedulerIntervalMinutes}m</span>
            <button type="button" class="btn btn-small btn-secondary" onclick={() => nudgeSchedulerInterval(1)}>+</button>
          </div>
        </div>
          <input
            class="threshold-slider"
            type="range"
            min="10"
            max="300"
            step="1"
            value={usageSchedulerIntervalMinutes}
            oninput={(event) => onSetUsageSchedulerIntervalInput(event.currentTarget.value)}
            onmouseup={(event) => onCommitUsageSchedulerIntervalInput(event.currentTarget.value)}
            onchange={(event) => onCommitUsageSchedulerIntervalInput(event.currentTarget.value)}
            aria-label="Usage scheduler interval minutes"
          />
          <div class="slider-scale">
            <span>10m</span>
            <span>60m</span>
            <span>300m</span>
          </div>
        </div>
        <p class="setting-title">Current scheduler interval: {usageSchedulerIntervalMinutesInput} minutes</p>
        <p class="setting-title">Each cycle refreshes account usage in batches from the database-backed account pool.</p>
      </div>

      <div class="setting-row">
        <p class="setting-title">Active Account Auto-Switch Job</p>
        <p class="setting-title">Backend job ini berjalan tetap setiap 5 menit. Job ini hanya cek usage akun API active dan CLI active, lalu switch jika usage aktif turun di bawah threshold.</p>
      </div>

      <div class="setting-row">
        <p class="setting-title">Usage Alert Threshold (%)</p>
        <div class="slider-wrap">
        <div class="slider-head">
          <span class="setting-title">Alert when remaining usage is below</span>
          <div class="inline-actions">
            <button type="button" class="btn btn-small btn-secondary" onclick={() => nudgeAlert(-1)}>-</button>
            <span class="slider-value">{usageAlertThreshold}%</span>
            <button type="button" class="btn btn-small btn-secondary" onclick={() => nudgeAlert(1)}>+</button>
          </div>
        </div>
          <input
            class="threshold-slider"
            type="range"
            min="0"
            max="100"
            step="1"
            value={usageAlertThreshold}
            oninput={(event) => onSetUsageAlertThresholdInput(event.currentTarget.value)}
            onmouseup={(event) => onCommitUsageAlertThresholdInput(event.currentTarget.value)}
            onchange={(event) => onCommitUsageAlertThresholdInput(event.currentTarget.value)}
            aria-label="Usage alert threshold percent"
          />
          <div class="slider-scale">
            <span>0%</span>
            <span>50%</span>
            <span>100%</span>
          </div>
        </div>
        <p class="setting-title">Current alert threshold: {usageAlertThreshold}%</p>
      </div>

      <div class="setting-row">
        <p class="setting-title">Auto-Switch Threshold (%) for Active Accounts</p>
        <div class="slider-wrap">
        <div class="slider-head">
          <span class="setting-title">Auto-switch when remaining usage is below</span>
          <div class="inline-actions">
            <button type="button" class="btn btn-small btn-secondary" onclick={() => nudgeAutoSwitch(-1)}>-</button>
            <span class="slider-value">{usageAutoSwitchThreshold}%</span>
            <button type="button" class="btn btn-small btn-secondary" onclick={() => nudgeAutoSwitch(1)}>+</button>
          </div>
        </div>
          <input
            class="threshold-slider"
            type="range"
            min="0"
            max="100"
            step="1"
            value={usageAutoSwitchThreshold}
            oninput={(event) => onSetUsageAutoSwitchThresholdInput(event.currentTarget.value)}
            onmouseup={(event) => onCommitUsageAutoSwitchThresholdInput(event.currentTarget.value)}
            onchange={(event) => onCommitUsageAutoSwitchThresholdInput(event.currentTarget.value)}
            aria-label="Usage auto switch threshold percent"
          />
          <div class="slider-scale">
            <span>0%</span>
            <span>50%</span>
            <span>100%</span>
          </div>
        </div>
        <p class="setting-title">Current auto-switch threshold: {usageAutoSwitchThreshold}%</p>
        <p class="setting-title">Active auto-switch runs every 5 minutes, refreshes the current API and CLI active accounts directly from upstream, then switches only if the active usage is below this threshold.</p>
        <p class="setting-title">Backup selection uses database snapshots and only considers accounts with weekly usage at least 80% or 5h usage at least 80%.</p>
        <p class="setting-title">Default logic: alert at 5%, auto-switch at 15%.</p>
      </div>

      <div class="setting-row">
        <p class="setting-title">Notification Sound</p>
        <div class="setting-actions-grid">
          <input value={usageSoundEnabled ? 'Sound enabled for use/switch/alert events' : 'Sound disabled'} readonly disabled />
          <button class="btn btn-secondary" onclick={onToggleUsageSoundEnabled}>
            {#if usageSoundEnabled}Disable Sound{:else}Enable Sound{/if}
          </button>
        </div>
      </div>
    </section>

  </div>
</section>
