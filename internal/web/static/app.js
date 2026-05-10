(function () {
  'use strict';

  const appEl = document.getElementById('app');
  const crumbEl = document.getElementById('crumb');

  function esc(s) {
    if (s == null || s === undefined) return '';
    return String(s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function encSeg(s) {
    return encodeURIComponent(s);
  }

  function fmtBytes(n) {
    const x = Number(n);
    if (!isFinite(x) || x === 0) return x === 0 ? '0 B' : esc(n);
    const u = 1024;
    if (x >= u * u * u * u) return (x / (u * u * u * u)).toFixed(2) + ' TiB';
    if (x >= u * u * u) return (x / (u * u * u)).toFixed(2) + ' GiB';
    if (x >= u * u) return (x / (u * u)).toFixed(2) + ' MiB';
    if (x >= u) return (x / u).toFixed(2) + ' KiB';
    return x + ' B';
  }

  function fmtUptime(sec) {
    const s = Number(sec);
    if (!isFinite(s) || s < 0) return '—';
    if (s < 60) return Math.round(s) + ' s';
    const d = Math.floor(s / 86400);
    let r = s - d * 86400;
    const h = Math.floor(r / 3600);
    r -= h * 3600;
    const m = Math.floor(r / 60);
    if (d > 0) return d + 'd ' + h + 'h ' + m + 'm';
    if (h > 0) return h + 'h ' + m + 'm';
    return m + 'm';
  }

  function fmtARCval(v) {
    const x = Number(v);
    if (!isFinite(x)) return esc(v);
    if (x >= 1024 * 1024) return fmtBytes(x);
    return String(Math.round(x));
  }

  async function j(url) {
    const r = await fetch(url);
    if (!r.ok) {
      let t = await r.text();
      try {
        const o = JSON.parse(t);
        if (o.error) t = o.error;
      } catch (_) {}
      throw new Error(t || r.statusText);
    }
    return r.json();
  }

  /** @returns {{ kind: string, parts: string[] }} */
  function parseRoute() {
    const raw = (location.hash || '').replace(/^#/, '').trim() || '/';
    const path = raw.startsWith('/') ? raw : '/' + raw;
    const segments = path
      .split('/')
      .filter(Boolean)
      .map(function (s) {
        try {
          return decodeURIComponent(s);
        } catch (_) {
          return s;
        }
      });
    if (segments.length === 0) return { kind: 'home', parts: [] };
    if (segments[0] === 'host') return { kind: 'host', parts: [] };
    if (segments[0] === 'pool' && segments[1]) {
      const pool = segments[1];
      if (segments[2] === 'disk' && segments[3] != null) {
        return { kind: 'disk', parts: [pool, segments.slice(3).join('/')] };
      }
      if (segments[2] === 'dataset' && segments[3] != null) {
        return { kind: 'dataset', parts: [pool, segments.slice(3).join('/')] };
      }
      if (segments[2] === 'zvol' && segments[3] != null) {
        return { kind: 'zvol', parts: [pool, segments.slice(3).join('/')] };
      }
      if (segments[2] === 'vdev' && segments[3] != null) {
        const idx = parseInt(segments[3], 10);
        if (isFinite(idx)) return { kind: 'vdev', parts: [pool, String(idx)] };
      }
      return { kind: 'pool', parts: [pool] };
    }
    return { kind: 'home', parts: [] };
  }

  function renderBreadcrumbs(items) {
    if (!items || !items.length) {
      crumbEl.classList.add('crumb--hidden');
      crumbEl.innerHTML = '';
      return;
    }
    crumbEl.classList.remove('crumb--hidden');
    crumbEl.innerHTML = items
      .map(function (it, i) {
        let piece;
        if (it.hash == null) {
          piece = '<span class="crumb-here">' + esc(it.label) + '</span>';
        } else {
          piece =
            '<a class="crumb-link" href="#' + esc(it.hash) + '">' + esc(it.label) + '</a>';
        }
        return (i > 0 ? '<span class="crumb-sep">›</span> ' : '') + piece;
      })
      .join(' ');
  }

  function githubReferenceTile() {
    var url = (document.body.getAttribute('data-github-repo') || '').trim();
    if (url) {
      return (
        '<a class="ref-tile" href="' +
        esc(url) +
        '" target="_blank" rel="noopener noreferrer">' +
        '<span class="ref-tile-kicker">External</span>' +
        '<span class="ref-tile-title">Source repository</span>' +
        '<span class="ref-tile-desc">Documentation, changes, and issue tracking on GitHub.</span>' +
        '</a>'
      );
    }
    return (
      '<div class="ref-tile ref-tile--muted">' +
      '<span class="ref-tile-kicker">External</span>' +
      '<span class="ref-tile-title">Source repository</span>' +
      '<span class="ref-tile-desc">No public source URL is bundled with this build. ' +
      'If your distribution ships one, use its documentation or support channels.</span>' +
      '</div>'
    );
  }

  function guessVdevRole(name) {
    const n = (name || '').toLowerCase();
    if (n.indexOf('draid') >= 0) return 'dRAID';
    if (n.indexOf('raidz') >= 0) return 'RAIDZ';
    if (n.indexOf('mirror') >= 0) return 'mirror';
    if (n === 'spares' || n.indexOf('spare') >= 0) return 'spare';
    if (n === 'logs' || n.indexOf('log') >= 0) return 'log';
    if (n === 'cache' || n.indexOf('cache') >= 0) return 'cache';
    if (n === 'special' || n.indexOf('special') >= 0) return 'special';
    if (n === 'dedup' || n.indexOf('dedup') >= 0) return 'dedup';
    if (n.indexOf('replacing') >= 0) return 'replacing';
    if (name && name.indexOf('/dev/') === 0) return 'disk';
    return 'vdev';
  }

  function enrichConfig(config, poolName) {
    const stack = [];
    return config.map(function (ln) {
      while (stack.length && stack[stack.length - 1].indent >= ln.indent) stack.pop();
      const ancestors = stack.map(function (s) {
        return s.name;
      });
      stack.push({ indent: ln.indent, name: ln.name });
      const vdevPath = ancestors.length ? ancestors.join(' › ') : poolName;
      const role = guessVdevRole(ln.name);
      const isDisk = ln.name.indexOf('/dev/') === 0;
      return Object.assign({}, ln, { ancestors: ancestors, vdevPath: vdevPath, role: role, isDisk: isDisk });
    });
  }

  function configChildren(config, lineIdx) {
    const base = config[lineIdx];
    if (!base) return [];
    const out = [];
    for (let i = lineIdx + 1; i < config.length; i++) {
      if (config[i].indent <= base.indent) break;
      out.push({ idx: i, line: config[i] });
    }
    return out;
  }

  function diskHref(pool, dev) {
    return '#/pool/' + encSeg(pool) + '/disk/' + encSeg(dev);
  }

  function vdevHref(pool, idx) {
    return '#/pool/' + encSeg(pool) + '/vdev/' + String(idx);
  }

  function datasetHref(pool, name) {
    return '#/pool/' + encSeg(pool) + '/dataset/' + encSeg(name);
  }

  function zvolHref(pool, name) {
    return '#/pool/' + encSeg(pool) + '/zvol/' + encSeg(name);
  }

  function smartURL(dev) {
    const seg = dev.indexOf('/dev/') === 0 ? dev : '/dev/' + dev;
    return '/v1/disk/' + encSeg(seg) + '/smart';
  }

  /** Shown when SMART/smartctl fails (missing binary, permissions, etc.). */
  function smartInstallHintHtml() {
    return (
      '<p class="muted small smart-install-hint">If <code class="inline-code">smartctl</code> is not installed, on Debian or Ubuntu install the package with ' +
      '<code class="inline-code">apt install smartmontools</code> (then retry this screen).</p>'
    );
  }

  function summarizeSmart(json) {
    if (!json || typeof json !== 'object') return [];
    const rows = [];
    const pick = [
      ['model_name', 'Model'],
      ['serial_number', 'Serial'],
      ['firmware_version', 'Firmware'],
      ['smartctl.version.0.string', 'smartctl'],
    ];
    for (let i = 0; i < pick.length; i++) {
      const k = pick[i][0];
      const label = pick[i][1];
      const parts = k.split('.');
      let v = json;
      for (let p = 0; p < parts.length && v; p++) v = v[parts[p]];
      if (v != null && typeof v !== 'object') rows.push([label, String(v)]);
    }
    if (json.user_capacity && json.user_capacity.bytes != null) {
      rows.push(['Capacity', fmtBytes(json.user_capacity.bytes)]);
    }
    if (json.smart_status) {
      const passed = json.smart_status.passed;
      if (passed != null) rows.push(['SMART status', passed ? 'passed' : 'failed']);
    }
    if (json.temperature && json.temperature.current != null) {
      rows.push(['Temperature', String(json.temperature.current) + ' °C']);
    }
    return rows;
  }

  function renderKV(rows) {
    if (!rows.length) return '<p class="muted">No details.</p>';
    return (
      '<dl class="kv">' +
      rows
        .map(function (r) {
          return '<dt>' + esc(r[0]) + '</dt><dd>' + esc(r[1]) + '</dd>';
        })
        .join('') +
      '</dl>'
    );
  }

  let poolsCache = null;

  async function getPools() {
    if (poolsCache) return poolsCache;
    poolsCache = await j('/v1/pools');
    return poolsCache;
  }

  async function renderHome() {
    renderBreadcrumbs(null);
    appEl.innerHTML = '<p class="loading">Loading…</p>';
    try {
      const pools = await getPools();
      const counts = await Promise.all(
        pools.map(function (p) {
          return j('/v1/pools/' + encSeg(p.name) + '/devices')
            .then(function (d) {
              return Array.isArray(d) ? d.length : 0;
            })
            .catch(function () {
              return '—';
            });
        })
      );
      const cards = pools
        .map(function (p, i) {
          const usedPct =
            p.size > 0 ? Math.round((100 * p.allocated) / p.size) : 0;
          return (
            '<a class="pool-card" href="#/pool/' +
            encSeg(p.name) +
            '">' +
            '<div class="pool-card-title">' +
            esc(p.name) +
            '</div>' +
            '<div class="pool-card-meta">' +
            '<span class="tag tag-health">' +
            esc(p.health) +
            '</span>' +
            '<span class="tag">' +
            esc(counts[i]) +
            ' disks</span>' +
            '</div>' +
            '<div class="pool-card-stats">' +
            '<span>' +
            fmtBytes(p.allocated) +
            ' used</span> · <span>' +
            fmtBytes(p.size) +
            ' total</span> · <span>' +
            usedPct +
            '%</span>' +
            '</div>' +
            '</a>'
          );
        })
        .join('');
      const poolBlock =
        pools.length > 0
          ? '<div class="pool-grid">' + cards + '</div>'
          : '<p class="muted">No storage pools were reported on this system.</p>';
      appEl.innerHTML =
        '<p class="home-lede">Read-only view of ZFS pools, topology, datasets, block volumes, and disks. ' +
        'Open a pool to drill down into vdevs, datasets, and SMART data.</p>' +
        '<section class="page-section" aria-labelledby="ref-heading">' +
        '<h2 id="ref-heading" class="section-heading">Reference</h2>' +
        '<div class="ref-grid">' +
        '<a class="ref-tile" href="#/host">' +
        '<span class="ref-tile-kicker">System</span>' +
        '<span class="ref-tile-title">Host overview</span>' +
        '<span class="ref-tile-desc">Hostname, operating system, kernel, memory, ARC statistics, and installed ZFS components.</span>' +
        '</a>' +
        '<div class="ref-tile ref-tile--static">' +
        '<span class="ref-tile-kicker">Application</span>' +
        '<span class="ref-tile-title">About zfstool</span>' +
        '<span class="ref-tile-desc">This interface collects data with zfs(8), zpool(8), and related read-only queries. ' +
        'It does not create, destroy, or modify pools or datasets.</span>' +
        '</div>' +
        githubReferenceTile() +
        '</div></section>' +
        '<section class="page-section" aria-labelledby="pools-heading">' +
        '<h2 id="pools-heading" class="section-heading">Storage pools</h2>' +
        '<p class="section-sub">Choose a pool to inspect configuration, datasets, zvols, and devices.</p>' +
        poolBlock +
        '</section>';
    } catch (e) {
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  async function renderHost() {
    renderBreadcrumbs([
      { label: 'Home', hash: '/' },
      { label: 'Host overview', hash: null },
    ]);
    appEl.innerHTML = '<p class="loading">Loading host…</p>';
    try {
      const h = await j('/v1/host');
      const os = [h.osName, h.osVersion].filter(Boolean).join(' ');
      let arcBlock = '';
      if (h.arc && Object.keys(h.arc).length) {
        const rows = Object.keys(h.arc)
          .sort()
          .map(function (k) {
            return (
              '<tr><th>' +
              esc(k) +
              '</th><td>' +
              fmtARCval(h.arc[k]) +
              '</td></tr>'
            );
          })
          .join('');
        arcBlock =
          '<h3 class="sub">ARC</h3><table class="arc">' + rows + '</table>';
      }
      const zfs =
        h.zfsVersions && h.zfsVersions.length
          ? h.zfsVersions.map(function (v) {
              return esc(v);
            }).join('<br>')
          : '—';
      const mem =
        h.memTotalKb > 0
          ? fmtBytes(h.memTotalKb * 1024) +
            ' total · ' +
            fmtBytes(h.memAvailableKb * 1024) +
            ' available'
          : '—';
      appEl.innerHTML =
        '<div class="card inner">' +
        '<dl class="kv">' +
        '<dt>Hostname</dt><dd>' +
        esc(h.hostname) +
        '</dd>' +
        '<dt>OS</dt><dd>' +
        (os ? esc(os) : '—') +
        '</dd>' +
        '<dt>Kernel</dt><dd>' +
        esc(h.kernel || '—') +
        '</dd>' +
        '<dt>Architecture</dt><dd>' +
        esc(h.arch || '—') +
        '</dd>' +
        '<dt>Uptime</dt><dd>' +
        esc(fmtUptime(h.uptimeSeconds)) +
        ' <span class="muted">(' +
        Math.round(h.uptimeSeconds) +
        ' s)</span></dd>' +
        '<dt>Memory</dt><dd>' +
        mem +
        '</dd>' +
        '<dt>ZFS</dt><dd>' +
        zfs +
        '</dd>' +
        (h.zfsMismatch
          ? '<dt></dt><dd class="err">User/kernel ZFS mismatch</dd>'
          : '') +
        '<dt>zfstool</dt><dd>' +
        esc(h.agentVersion || '—') +
        '</dd>' +
        '<dt>Collected</dt><dd>' +
        esc(h.collectedAt || '—') +
        '</dd>' +
        '</dl>' +
        arcBlock +
        '</div>';
    } catch (e) {
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  async function renderPool(poolName) {
    renderBreadcrumbs([
      { label: 'Home', hash: '/' },
      { label: poolName, hash: null },
    ]);
    appEl.innerHTML = '<p class="loading">Loading pool…</p>';
    try {
      const [summary, st, datasets] = await Promise.all([
        getPools().then(function (ps) {
          return ps.find(function (x) {
            return x.name === poolName;
          });
        }),
        j('/v1/pools/' + encSeg(poolName) + '/status'),
        j('/v1/datasets?pool=' + encSeg(poolName)),
      ]);
      const enriched = enrichConfig(st.config || [], poolName);
      const disks = enriched.filter(function (l) {
        return l.isDisk;
      });
      const filesystems = datasets.filter(function (d) {
        return d.type === 'filesystem';
      });
      const volumes = datasets.filter(function (d) {
        return d.type === 'volume';
      });
      const snapshots = datasets.filter(function (d) {
        return d.type === 'snapshot';
      });

      let scanLine = '';
      if (st.scan && st.scan.raw) {
        scanLine =
          '<p class="scan-line"><strong>Scan</strong> · ' +
          esc(st.scan.raw) +
          '</p>';
      } else if (st.scanRaw) {
        scanLine =
          '<p class="scan-line"><strong>Scan</strong> · ' + esc(st.scanRaw) + '</p>';
      }

      const treeRows = enriched
        .map(function (ln, idx) {
          const pad = Math.min(ln.indent * 0.55, 24);
          const roleTag =
            '<span class="tag tag-role">' + esc(ln.role) + '</span>';
          let link = esc(ln.name);
          if (ln.isDisk) {
            link =
              '<a href="' +
              esc(diskHref(poolName, ln.name)) +
              '">' +
              esc(ln.name) +
              '</a>';
          } else if (ln.name !== poolName) {
            link =
              '<a href="' +
              esc(vdevHref(poolName, idx)) +
              '">' +
              esc(ln.name) +
              '</a>';
          }
          return (
            '<tr class="tree-row">' +
            '<td class="tree-name" style="padding-left:' +
            pad +
            'rem">' +
            link +
            ' ' +
            roleTag +
            '</td>' +
            '<td>' +
            esc(ln.state || '—') +
            '</td>' +
            '<td class="mono">' +
            esc(ln.read || '—') +
            '</td>' +
            '<td class="mono">' +
            esc(ln.write || '—') +
            '</td>' +
            '<td class="mono">' +
            esc(ln.cksum || '—') +
            '</td>' +
            '</tr>'
          );
        })
        .join('');

      const fsRows = filesystems
        .map(function (d) {
          return (
            '<tr><td><a href="' +
            esc(datasetHref(poolName, d.name)) +
            '">' +
            esc(d.name) +
            '</a></td><td>' +
            fmtBytes(d.used) +
            '</td><td>' +
            fmtBytes(d.avail) +
            '</td><td>' +
            esc(d.mountpoint || '—') +
            '</td></tr>'
          );
        })
        .join('');

      const volRows = volumes
        .map(function (d) {
          return (
            '<tr><td><a href="' +
            esc(zvolHref(poolName, d.name)) +
            '">' +
            esc(d.name) +
            '</a></td><td>' +
            fmtBytes(d.used) +
            '</td><td>' +
            fmtBytes(d.refer) +
            '</td></tr>'
          );
        })
        .join('');

      const sum = summary || {};
      const cap =
        sum.size > 0 ? Math.round((100 * sum.allocated) / sum.size) : 0;

      appEl.innerHTML =
        '<div class="card inner pool-summary">' +
        '<dl class="kv">' +
        '<dt>State</dt><dd>' +
        esc(st.state) +
        '</dd>' +
        '<dt>Health</dt><dd>' +
        esc(sum.health || '—') +
        '</dd>' +
        '<dt>Size</dt><dd>' +
        fmtBytes(sum.size) +
        '</dd>' +
        '<dt>Allocated</dt><dd>' +
        fmtBytes(sum.allocated) +
        ' (' +
        cap +
        '%)</dd>' +
        '<dt>Free</dt><dd>' +
        fmtBytes(sum.free) +
        '</dd>' +
        '<dt>Disks</dt><dd>' +
        String(disks.length) +
        '</dd>' +
        (st.errors
          ? '<dt>Errors</dt><dd class="err">' + esc(st.errors) + '</dd>'
          : '') +
        '</dl>' +
        scanLine +
        '</div>' +
        '<h3 class="sub">Topology</h3>' +
        '<div class="table-wrap">' +
        '<table class="tree-table">' +
        '<thead><tr><th>Name</th><th>State</th><th>Read</th><th>Write</th><th>Cksum</th></tr></thead>' +
        '<tbody>' +
        treeRows +
        '</tbody></table></div>' +
        '<h3 class="sub">Filesystems (' +
        filesystems.length +
        ')</h3>' +
        '<div class="table-wrap">' +
        '<table><thead><tr><th>Name</th><th>Used</th><th>Avail</th><th>Mount</th></tr></thead><tbody>' +
        (fsRows || '<tr><td colspan="4" class="muted">None</td></tr>') +
        '</tbody></table></div>' +
        '<h3 class="sub">Volumes / zvols (' +
        volumes.length +
        ')</h3>' +
        '<div class="table-wrap">' +
        '<table><thead><tr><th>Name</th><th>Used</th><th>Refer</th></tr></thead><tbody>' +
        (volRows || '<tr><td colspan="3" class="muted">None</td></tr>') +
        '</tbody></table></div>' +
        '<h3 class="sub">Snapshots (' +
        snapshots.length +
        ')</h3>' +
        '<div class="table-wrap">' +
        '<table><thead><tr><th>Name</th><th>Used</th><th>Refer</th></tr></thead><tbody>' +
        (snapshots.length
          ? snapshots
              .slice(0, 60)
              .map(function (d) {
                return (
                  '<tr><td>' +
                  esc(d.name) +
                  '</td><td>' +
                  fmtBytes(d.used) +
                  '</td><td>' +
                  fmtBytes(d.refer) +
                  '</td></tr>'
                );
              })
              .join('') +
            (snapshots.length > 60
              ? '<tr><td colspan="3" class="muted">… ' +
                (snapshots.length - 60) +
                ' more not shown</td></tr>'
              : '')
          : '<tr><td colspan="3" class="muted">None</td></tr>') +
        '</tbody></table></div>';
    } catch (e) {
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  async function renderVdev(poolName, idxStr) {
    const idx = parseInt(idxStr, 10);
    appEl.innerHTML = '<p class="loading">Loading…</p>';
    try {
      const st = await j('/v1/pools/' + encSeg(poolName) + '/status');
      const enriched = enrichConfig(st.config || [], poolName);
      const ln = enriched[idx];
      if (!ln) throw new Error('Vdev line not found');
      const kids = configChildren(enriched, idx);
      renderBreadcrumbs([
        { label: 'Home', hash: '/' },
        { label: poolName, hash: '/pool/' + encSeg(poolName) },
        { label: ln.name, hash: null },
      ]);
      const childRows = kids
        .map(function (k) {
          const c = k.line;
          const nm = c.isDisk
            ? '<a href="' + esc(diskHref(poolName, c.name)) + '">' + esc(c.name) + '</a>'
            : '<a href="' + esc(vdevHref(poolName, k.idx)) + '">' + esc(c.name) + '</a>';
          return (
            '<tr><td>' +
            nm +
            '</td><td>' +
            esc(c.role) +
            '</td><td>' +
            esc(c.state) +
            '</td></tr>'
          );
        })
        .join('');
      appEl.innerHTML =
        '<div class="card inner">' +
        '<p><span class="tag tag-role">' +
        esc(ln.role) +
        '</span></p>' +
        renderKV([
          ['Name', ln.name],
          ['Under', ln.vdevPath],
          ['State', ln.state || '—'],
          ['READ', ln.read || '—'],
          ['WRITE', ln.write || '—'],
          ['CKSUM', ln.cksum || '—'],
        ]) +
        '</div>' +
        '<h3 class="sub">Children</h3>' +
        '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Role</th><th>State</th></tr></thead><tbody>' +
        (childRows || '<tr><td colspan="3" class="muted">None</td></tr>') +
        '</tbody></table></div>';
    } catch (e) {
      renderBreadcrumbs([
        { label: 'Home', hash: '/' },
        { label: poolName, hash: '/pool/' + encSeg(poolName) },
        { label: 'Vdev', hash: null },
      ]);
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  async function renderDisk(poolName, devPath) {
    renderBreadcrumbs([
      { label: 'Home', hash: '/' },
      { label: poolName, hash: '/pool/' + encSeg(poolName) },
      { label: devPath.replace(/^.*\//, '') || devPath, hash: null },
    ]);
    appEl.innerHTML = '<p class="loading">Loading disk…</p>';
    try {
      const st = await j('/v1/pools/' + encSeg(poolName) + '/status');
      const enriched = enrichConfig(st.config || [], poolName);
      const hit = enriched.find(function (l) {
        return l.isDisk && l.name === devPath;
      });
      const smart = await j(smartURL(devPath)).catch(function (e) {
        return { error: e.message || String(e), json: null };
      });
      const sj = smart && smart.json;
      const hasJson = sj && typeof sj === 'object' && Object.keys(sj).length > 0;
      let smartBlock = '';
      if (hasJson) {
        const summary = summarizeSmart(sj);
        smartBlock =
          '<h3 class="sub">SMART</h3>' +
          (smart.error
            ? '<p class="err small">Warning: ' + esc(smart.error) + '</p>' + smartInstallHintHtml()
            : '') +
          renderKV(summary) +
          '<details class="raw-smart"><summary>Raw JSON</summary><pre>' +
          esc(JSON.stringify(smart, null, 2)) +
          '</pre></details>';
      } else if (smart && smart.error) {
        smartBlock =
          '<h3 class="sub">SMART</h3><p class="err">' + esc(smart.error) + '</p>' +
          smartInstallHintHtml() +
          '<details class="raw-smart"><summary>Response</summary><pre>' +
          esc(JSON.stringify(smart, null, 2)) +
          '</pre></details>';
      } else {
        smartBlock = '<h3 class="sub">SMART</h3><p class="muted">No SMART data.</p>';
      }
      const rows = hit
        ? [
            ['Device', hit.name],
            ['Pool', poolName],
            ['Topology path', hit.vdevPath],
            ['Role', hit.role],
            ['State', hit.state || '—'],
            ['READ', hit.read || '—'],
            ['WRITE', hit.write || '—'],
            ['CKSUM', hit.cksum || '—'],
          ]
        : [
            ['Device', devPath],
            ['Pool', poolName],
            ['Topology path', 'Not found in current status (detached or renamed?)'],
          ];
      appEl.innerHTML =
        '<div class="card inner">' +
        '<h3 class="sub">In pool</h3>' +
        renderKV(rows) +
        '</div>' +
        smartBlock;
    } catch (e) {
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  async function renderDatasetLike(kind, poolName, dsName) {
    const label = kind === 'zvol' ? 'Zvol' : 'Dataset';
    renderBreadcrumbs([
      { label: 'Home', hash: '/' },
      { label: poolName, hash: '/pool/' + encSeg(poolName) },
      { label: dsName, hash: null },
    ]);
    appEl.innerHTML = '<p class="loading">Loading ' + label.toLowerCase() + '…</p>';
    try {
      const data = await j('/v1/datasets/properties?name=' + encSeg(dsName));
      const props = data.properties || {};
      const keys = Object.keys(props).sort();
      const interesting = [
        'type',
        'used',
        'available',
        'referenced',
        'compressratio',
        'compression',
        'dedup',
        'mountpoint',
        'canmount',
        'readonly',
        'recordsize',
        'volsize',
        'volblocksize',
        'origin',
        'createtxg',
        'creation',
      ];
      const priority = [];
      for (let i = 0; i < interesting.length; i++) {
        const k = interesting[i];
        if (props[k] != null) priority.push([k, props[k]]);
      }
      const rest = keys.filter(function (k) {
        return interesting.indexOf(k) < 0;
      });
      const rows = priority.concat(
        rest.map(function (k) {
          return [k, props[k]];
        })
      );
      appEl.innerHTML =
        '<div class="card inner">' +
        '<p><span class="tag">' +
        esc(label) +
        '</span> <span class="tag tag-role">' +
        esc(props.type || kind) +
        '</span></p>' +
        renderKV(rows) +
        '</div>';
    } catch (e) {
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  function navigateRoute(r) {
    switch (r.kind) {
      case 'home':
        renderHome();
        break;
      case 'host':
        renderHost();
        break;
      case 'pool':
        renderPool(r.parts[0]);
        break;
      case 'vdev':
        renderVdev(r.parts[0], r.parts[1]);
        break;
      case 'disk':
        renderDisk(r.parts[0], r.parts[1]);
        break;
      case 'dataset':
        renderDatasetLike('dataset', r.parts[0], r.parts[1]);
        break;
      case 'zvol':
        renderDatasetLike('zvol', r.parts[0], r.parts[1]);
        break;
      default:
        renderHome();
    }
  }

  function dispatch() {
    navigateRoute(parseRoute());
  }

  function refreshCurrentView() {
    poolsCache = null;
    navigateRoute(parseRoute());
  }

  function invokeNativeExit() {
    if (typeof window.zfstoolExit !== 'function') return;
    try {
      var p = window.zfstoolExit();
      if (p && typeof p.then === 'function') {
        p.catch(function () {});
      }
    } catch (_) {}
  }

  window.addEventListener('hashchange', dispatch);
  dispatch();

  document.addEventListener(
    'keydown',
    function (e) {
      if (e.key === 'F5') {
        e.preventDefault();
        refreshCurrentView();
        return;
      }
      if (
        e.ctrlKey &&
        !e.shiftKey &&
        !e.altKey &&
        (e.key === 'q' || e.key === 'Q' || e.key === 'w' || e.key === 'W')
      ) {
        if (typeof window.zfstoolExit === 'function') {
          e.preventDefault();
          invokeNativeExit();
        }
      }
    },
    true
  );

  function wireNativeExit() {
    var btn = document.getElementById('btn-native-exit');
    if (!btn || btn.dataset.wired === '1') return;
    if (typeof window.zfstoolExit !== 'function') return;
    btn.hidden = false;
    btn.dataset.wired = '1';
    btn.addEventListener('click', function () {
      invokeNativeExit();
    });
  }
  wireNativeExit();
  setTimeout(wireNativeExit, 0);
  setTimeout(wireNativeExit, 100);
})();
