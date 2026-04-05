<script>
  import { onMount } from 'svelte';

  let { monitorSnapshot, onRefreshMonitor } = $props();

  function formatBytes(value) {
    const n = Number(value || 0);
    if (!Number.isFinite(n) || n <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let x = n;
    let i = 0;
    while (x >= 1024 && i < units.length - 1) {
      x /= 1024;
      i += 1;
    }
    return `${x.toFixed(x >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
  }

  function formatPct(value) {
    const n = Number(value || 0);
    if (!Number.isFinite(n)) return '0%';
    return `${n.toFixed(n >= 10 ? 0 : 1)}%`;
  }

  function formatUsage(used, total, pct) {
    const t = Number(total || 0);
    if (!Number.isFinite(t) || t <= 0) return 'N/A';
    return `${formatBytes(used)} / ${formatBytes(total)} (${formatPct(pct)})`;
  }

  function formatLoadAvg(host) {
    if (!host || host.load_supported === false) return 'N/A (Windows)';
    return `${host.load_1 ?? 0} / ${host.load_5 ?? 0} / ${host.load_15 ?? 0}`;
  }

  onMount(() => {
    let timer = null;
    const tick = () => {
      if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return;
      onRefreshMonitor({ quiet: true });
    };
    tick();
    timer = setInterval(tick, 1000);
    return () => {
      if (timer) clearInterval(timer);
    };
  });
</script>

<section class="panel">
  <div class="panel-header panel-header-inline">
    <h2>Monitor</h2>
  </div>

  {#if monitorSnapshot?.ok}
    <div class="accounts-grid">
      <article class="account-card">
        <div class="account-head">
          <div><p class="account-email">CodexSess Process</p></div>
          <span class="account-state state-active">LIVE</span>
        </div>
        {#if monitorSnapshot?.process?.metrics_available === false}
          <p class="usage-reset">Detailed process counters unavailable on this host.</p>
        {/if}
        <div class="usage-list">
          <div class="usage-item"><div class="usage-top"><p>CPU</p><p>{formatPct(monitorSnapshot?.process?.cpu_percent)}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>RSS</p><p>{formatBytes(monitorSnapshot?.process?.rss_bytes)}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>VSize</p><p>{formatBytes(monitorSnapshot?.process?.vsize_bytes)}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>Threads</p><p>{monitorSnapshot?.process?.threads ?? '-'}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>Goroutines</p><p>{monitorSnapshot?.process?.goroutines ?? '-'}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>Uptime</p><p>{Math.round(Number(monitorSnapshot?.process?.uptime_seconds || 0))}s</p></div></div>
        </div>
      </article>

      <article class="account-card">
        <div class="account-head">
          <div><p class="account-email">Host ({monitorSnapshot?.host?.hostname || '-'})</p></div>
          <span class="account-state">VPS</span>
        </div>
        {#if monitorSnapshot?.host?.metrics_available === false}
          <p class="usage-reset">Host counters are not available. Rebuild and restart CodexSess binary.</p>
        {/if}
        <div class="usage-list">
          <div class="usage-item"><div class="usage-top"><p>CPU</p><p>{formatPct(monitorSnapshot?.host?.cpu_percent)}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>Load Avg</p><p>{formatLoadAvg(monitorSnapshot?.host)}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>Memory</p><p>{formatUsage(monitorSnapshot?.host?.memory_used_bytes, monitorSnapshot?.host?.memory_total_bytes, monitorSnapshot?.host?.memory_used_pct)}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>Disk</p><p>{formatUsage(monitorSnapshot?.host?.disk_used_bytes, monitorSnapshot?.host?.disk_total_bytes, monitorSnapshot?.host?.disk_used_pct)}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>Uptime</p><p>{Math.round(Number(monitorSnapshot?.host?.uptime_seconds || 0))}s</p></div></div>
        </div>
      </article>

      <article class="account-card">
        <div class="account-head">
          <div><p class="account-email">Go Runtime</p></div>
          <span class="account-state">RUNTIME</span>
        </div>
        <div class="usage-list">
          <div class="usage-item"><div class="usage-top"><p>Version</p><p>{monitorSnapshot?.go_runtime?.version || '-'}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>Heap Alloc</p><p>{formatBytes(monitorSnapshot?.go_runtime?.heap_alloc_bytes)}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>Heap Sys</p><p>{formatBytes(monitorSnapshot?.go_runtime?.heap_sys_bytes)}</p></div></div>
          <div class="usage-item"><div class="usage-top"><p>GC Cycles</p><p>{monitorSnapshot?.go_runtime?.gc_cycles ?? '-'}</p></div></div>
        </div>
      </article>
    </div>
  {:else}
    <div class="empty-state">Monitor data not available.</div>
  {/if}
</section>
