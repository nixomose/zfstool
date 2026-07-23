(function () {
  'use strict';

  const appEl = document.getElementById('app');
  const crumbEl = document.getElementById('crumb');
  const sidebarEl = document.getElementById('sidebar-nav');
  const connPill = document.getElementById('conn-pill');
  const topbarMeta = document.getElementById('topbar-meta');

  const nav = {
    poolsOpen: true,
    disksOpen: true,
    poolKids: {},
    diskKids: {},
    smart: {},
  };

  let poolsCache = null;
  let disksCache = null;
  let hostCache = null;
  let sidebarBuiltFor = '';

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

  function shortDev(path) {
    const s = String(path || '');
    const i = s.lastIndexOf('/');
    return i >= 0 ? s.slice(i + 1) : s;
  }

  function poolOfDataset(name) {
    if (!name) return '';
    return String(name).split('/')[0].split('@')[0];
  }

  async function j(url, opts) {
    const r = await fetch(url, opts);
    if (!r.ok) {
      let t = await r.text();
      try {
        const o = JSON.parse(t);
        if (o.error) t = o.error;
      } catch (_) {}
      throw new Error(t || r.statusText);
    }
    if (r.status === 204) return null;
    return r.json();
  }

  function parseRoute() {
    const raw = (location.hash || '').replace(/^#/, '').trim() || '/';
    const path = raw.startsWith('/') ? raw : '/' + raw;
    const qIdx = path.indexOf('?');
    const pathOnly = qIdx >= 0 ? path.slice(0, qIdx) : path;
    const query = {};
    if (qIdx >= 0) {
      path
        .slice(qIdx + 1)
        .split('&')
        .forEach(function (pair) {
          const kv = pair.split('=');
          if (kv[0]) {
            try {
              query[decodeURIComponent(kv[0])] = decodeURIComponent(kv[1] || '');
            } catch (_) {
              query[kv[0]] = kv[1] || '';
            }
          }
        });
    }
    const segments = pathOnly
      .split('/')
      .filter(Boolean)
      .map(function (s) {
        try {
          return decodeURIComponent(s);
        } catch (_) {
          return s;
        }
      });
    if (segments.length === 0) return { kind: 'home', parts: [], query: query };
    if (segments[0] === 'host') return { kind: 'host', parts: [], query: query };
    if (segments[0] === 'remote') return { kind: 'remote', parts: [], query: query };
    if (segments[0] === 'disk' && segments[1] != null) {
      return { kind: 'disk', parts: [segments.slice(1).join('/')], query: query };
    }
    if (segments[0] === 'pool' && segments[1]) {
      const pool = segments[1];
      if (segments[2] === 'disk' && segments[3] != null) {
        return { kind: 'disk', parts: [segments.slice(3).join('/')], query: query, fromPool: pool };
      }
      if (segments[2] === 'dataset' && segments[3] != null) {
        return { kind: 'dataset', parts: [pool, segments.slice(3).join('/')], query: query };
      }
      if (segments[2] === 'zvol' && segments[3] != null) {
        return { kind: 'zvol', parts: [pool, segments.slice(3).join('/')], query: query };
      }
      if (segments[2] === 'vdev' && segments[3] != null) {
        const idx = parseInt(segments[3], 10);
        if (isFinite(idx)) return { kind: 'vdev', parts: [pool, String(idx)], query: query };
      }
      return { kind: 'pool', parts: [pool], query: query };
    }
    return { kind: 'home', parts: [], query: query };
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
          piece = '<a class="crumb-link" href="#' + esc(it.hash) + '">' + esc(it.label) + '</a>';
        }
        return (i > 0 ? '<span class="crumb-sep">›</span> ' : '') + piece;
      })
      .join(' ');
  }

  function diskHref(dev) {
    return '#/disk/' + encSeg(dev);
  }

  function poolHref(pool, tab) {
    var h = '#/pool/' + encSeg(pool);
    if (tab) h += '?tab=' + encSeg(tab);
    return h;
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

  function dsLink(name, typeHint) {
    if (!name) return '—';
    const pool = poolOfDataset(name);
    const isVol = typeHint === 'volume' || typeHint === 'zvol';
    const href = isVol ? zvolHref(pool, name) : datasetHref(pool, name);
    return '<a href="' + esc(href) + '">' + esc(name) + '</a>';
  }

  function poolLink(name) {
    if (!name) return '—';
    return '<a href="' + esc(poolHref(name)) + '">' + esc(name) + '</a>';
  }

  function diskLink(dev) {
    if (!dev) return '—';
    return '<a href="' + esc(diskHref(dev)) + '" title="' + esc(dev) + '">' + esc(shortDev(dev)) + '</a>';
  }

  function smartURL(dev) {
    const seg = String(dev).indexOf('/dev/') === 0 ? dev : '/dev/' + String(dev).replace(/^\//, '');
    return '/v1/disk/' + encSeg(seg) + '/smart';
  }

  function smartHintHtml(smart) {
    const kind = (smart && smart.error_kind) || '';
    if (kind === 'not_found') {
      return (
        '<p class="muted small">Install smartctl: ' +
        '<code class="inline-code">apt install smartmontools</code> ' +
        '(package name may differ on other distros).</p>'
      );
    }
    if (kind === 'permission') {
      return (
        '<p class="muted small">If not running as root, add your user to the ' +
        '<code class="inline-code">disk</code> group: ' +
        '<code class="inline-code">sudo usermod -aG disk $USER</code> ' +
        'then log out and back in.</p>'
      );
    }
    // smartctl is present but returned no usable SMART data — don't suggest apt install.
    return '';
  }

  function summarizeSmart(json) {
    if (!json || typeof json !== 'object') return [];
    const rows = [];
    const pick = [
      ['model_name', 'Model'],
      ['serial_number', 'Serial'],
      ['firmware_version', 'Firmware'],
    ];
    for (let i = 0; i < pick.length; i++) {
      const parts = pick[i][0].split('.');
      let v = json;
      for (let p = 0; p < parts.length && v; p++) v = v[parts[p]];
      if (v != null && typeof v !== 'object') rows.push([pick[i][1], String(v)]);
    }
    if (json.user_capacity && json.user_capacity.bytes != null) {
      rows.push(['Capacity', fmtBytes(json.user_capacity.bytes)]);
    }
    if (json.smart_status) {
      const passed = json.smart_status.passed;
      if (passed != null) rows.push(['SMART', passed ? 'passed' : 'failed']);
    }
    if (json.temperature && json.temperature.current != null) {
      rows.push(['Temp', String(json.temperature.current) + ' °C']);
    }
    const attrs = json.ata_smart_attributes && json.ata_smart_attributes.table;
    if (Array.isArray(attrs)) {
      attrs.forEach(function (a) {
        if (!a || !a.name) return;
        const n = String(a.name).toLowerCase();
        if (
          n.indexOf('reallocated') >= 0 ||
          n.indexOf('pending') >= 0 ||
          n.indexOf('uncorrectable') >= 0 ||
          n.indexOf('power_on') >= 0 ||
          n.indexOf('power_cycle') >= 0
        ) {
          const raw = a.raw && a.raw.value != null ? a.raw.value : a.value;
          rows.push([a.name, String(raw)]);
        }
      });
    }
    return rows;
  }

  function smartSnippetHtml(smart) {
    if (!smart) return '<span class="muted">…</span>';
    if (smart.error && !(smart.json && Object.keys(smart.json).length)) {
      return '<span class="bad">' + esc(smart.error) + '</span>';
    }
    const sj = smart.json || {};
    const bits = [];
    if (sj.smart_status && sj.smart_status.passed != null) {
      bits.push(
        '<span class="' +
          (sj.smart_status.passed ? 'ok' : 'bad') +
          '">' +
          (sj.smart_status.passed ? 'passed' : 'failed') +
          '</span>'
      );
    }
    if (sj.temperature && sj.temperature.current != null) {
      bits.push(esc(String(sj.temperature.current) + '°C'));
    }
    if (sj.model_name) bits.push(esc(sj.model_name));
    if (!bits.length) return '<span class="muted">no summary</span>';
    return bits.join(' · ');
  }

  function renderKV(rows, allowHtml) {
    if (!rows.length) return '<p class="muted">No details.</p>';
    return (
      '<dl class="kv">' +
      rows
        .map(function (r) {
          const v = allowHtml ? r[1] : esc(r[1]);
          return '<dt>' + esc(r[0]) + '</dt><dd>' + v + '</dd>';
        })
        .join('') +
      '</dl>'
    );
  }

  function filterBarHtml(placeholder) {
    return (
      '<div class="list-filter">' +
      '<input type="search" class="list-filter-input" placeholder="' +
      esc(placeholder || 'Filter…') +
      '" autocomplete="off" spellcheck="false" />' +
      '<span class="list-filter-count" hidden></span>' +
      '</div>'
    );
  }

  function applyListFilter(box, query) {
    const q = String(query || '')
      .trim()
      .toLowerCase();
    const countEl = box.querySelector('.list-filter-count');
    let total = 0;
    let shown = 0;

    const tbody = box.querySelector('table tbody');
    if (tbody) {
      Array.prototype.forEach.call(tbody.rows, function (tr) {
        total++;
        const match = !q || (tr.textContent || '').toLowerCase().indexOf(q) >= 0;
        tr.hidden = !match;
        if (match) shown++;
      });
    } else {
      const kv = box.querySelector('dl.kv');
      if (kv) {
        Array.prototype.forEach.call(kv.querySelectorAll('dt'), function (dt) {
          total++;
          const dd = dt.nextElementSibling;
          const text =
            (dt.textContent || '') + ' ' + (dd && dd.tagName === 'DD' ? dd.textContent || '' : '');
          const match = !q || text.toLowerCase().indexOf(q) >= 0;
          dt.hidden = !match;
          if (dd && dd.tagName === 'DD') dd.hidden = !match;
          if (match) shown++;
        });
      } else {
        Array.prototype.forEach.call(box.querySelectorAll('.filter-line'), function (line) {
          total++;
          const match = !q || (line.textContent || '').toLowerCase().indexOf(q) >= 0;
          line.hidden = !match;
          if (match) shown++;
        });
      }
    }

    if (countEl) {
      if (q && total > 0) {
        countEl.hidden = false;
        countEl.textContent = shown + '/' + total;
      } else {
        countEl.hidden = true;
        countEl.textContent = '';
      }
    }
  }

  function wireListFilters(root) {
    const scope = root || appEl;
    Array.prototype.forEach.call(scope.querySelectorAll('.filterable'), function (box) {
      const input = box.querySelector('.list-filter-input');
      if (!input || input.dataset.wired === '1') return;
      input.dataset.wired = '1';
      input.addEventListener('input', function () {
        applyListFilter(box, input.value);
      });
      if (input.value) applyListFilter(box, input.value);
    });
  }

  /** Attach substring search to every table / kv list / marked pre in the main pane. */
  function enhanceFilterableLists(root) {
    const scope = root || appEl;

    Array.prototype.forEach.call(scope.querySelectorAll('.table-wrap'), function (wrap) {
      if (wrap.closest('.filterable')) return;
      const table = wrap.querySelector('table');
      if (!table || !table.tBodies.length || !table.tBodies[0].rows.length) return;
      if (table.tBodies[0].rows.length < 2 && !(table.tBodies[0].rows[0].textContent || '').trim()) {
        return;
      }
      const box = document.createElement('div');
      box.className = 'filterable';
      wrap.parentNode.insertBefore(box, wrap);
      box.insertAdjacentHTML('afterbegin', filterBarHtml('Filter list…'));
      box.appendChild(wrap);
    });

    Array.prototype.forEach.call(scope.querySelectorAll('dl.kv'), function (kv) {
      if (kv.closest('.filterable')) return;
      const dts = kv.querySelectorAll('dt');
      if (dts.length < 2) return;
      const box = document.createElement('div');
      box.className = 'filterable';
      kv.parentNode.insertBefore(box, kv);
      box.insertAdjacentHTML('afterbegin', filterBarHtml('Filter fields…'));
      box.appendChild(kv);
    });

    Array.prototype.forEach.call(scope.querySelectorAll('pre'), function (pre) {
      if (pre.closest('.filterable')) return;
      if (pre.classList.contains('no-filter')) return;
      const raw = pre.textContent || '';
      if (!raw.trim() || raw.indexOf('\n') < 0) return;
      // Skip huge raw JSON dumps unless already line-oriented logs
      if (raw.charAt(0) === '{' || raw.charAt(0) === '[') return;
      const lines = raw.split('\n');
      if (lines.length < 3) return;
      const box = document.createElement('div');
      box.className = 'filterable';
      pre.parentNode.insertBefore(box, pre);
      box.insertAdjacentHTML('afterbegin', filterBarHtml('Filter lines…'));
      const host = document.createElement('pre');
      host.className = pre.className;
      host.innerHTML = lines
        .map(function (ln) {
          return '<div class="filter-line">' + esc(ln) + '</div>';
        })
        .join('');
      box.appendChild(host);
      pre.remove();
    });

    wireListFilters(scope);
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

  function looksLikeDisk(name) {
    if (!name) return false;
    const n = String(name);
    const lower = n.toLowerCase();
    if (
      lower === 'logs' ||
      lower === 'cache' ||
      lower === 'spares' ||
      lower === 'special' ||
      lower === 'dedup' ||
      lower.indexOf('mirror') === 0 ||
      lower.indexOf('raidz') >= 0 ||
      lower.indexOf('draid') >= 0 ||
      lower.indexOf('replacing') === 0
    ) {
      return false;
    }
    if (n.indexOf('/dev/') === 0) return true;
    if (
      /ata-|nvme-|wwn-|scsi-|usb-/.test(lower) ||
      /^(sd[a-z]+[0-9]*|vd[a-z]+[0-9]*|nvme[0-9]+n[0-9]+(p[0-9]+)?|md[0-9]+|dm-[0-9]+)$/.test(n)
    ) {
      return true;
    }
    return false;
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
      const isDisk = looksLikeDisk(ln.name);
      const role = isDisk ? 'disk' : guessVdevRole(ln.name);
      return Object.assign({}, ln, {
        ancestors: ancestors,
        vdevPath: vdevPath,
        role: role,
        isDisk: isDisk,
      });
    });
  }

  function cssAttrEscape(s) {
    if (typeof CSS !== 'undefined' && CSS.escape) return CSS.escape(s);
    return String(s).replace(/\\/g, '\\\\').replace(/"/g, '\\"');
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

  async function getPools(force) {
    if (!force && poolsCache) return poolsCache;
    poolsCache = await j('/v1/pools');
    return poolsCache;
  }

  async function getDisks(force) {
    if (!force && disksCache) return disksCache;
    disksCache = await j('/v1/disks');
    return disksCache;
  }

  async function getHost(force) {
    if (!force && hostCache) return hostCache;
    hostCache = await j('/v1/host');
    return hostCache;
  }

  async function updateConnectionChrome() {
    try {
      const h = await getHost();
      const host = h.hostname || 'local';
      connPill.innerHTML =
        '<a href="#/remote" title="Remoting / agent">' + esc(host) + '</a>';
      topbarMeta.textContent =
        (h.hostname || 'host') +
        ' · ' +
        (h.agentVersion || '?') +
        ' · ' +
        (h.osName || '') +
        (h.osVersion ? ' ' + h.osVersion : '');
    } catch (e) {
      connPill.textContent = 'offline';
      topbarMeta.textContent = 'Agent unreachable: ' + (e.message || e);
    }
  }

  function routeActive(r) {
    if (r.kind === 'pool' || r.kind === 'vdev' || r.kind === 'dataset' || r.kind === 'zvol') {
      return { type: 'pool', id: r.parts[0] };
    }
    if (r.kind === 'disk') return { type: 'disk', id: r.parts[0] };
    if (r.kind === 'host') return { type: 'host' };
    if (r.kind === 'remote') return { type: 'remote' };
    return { type: 'home' };
  }

  async function ensureDiskSmart(dev) {
    if (nav.smart[dev] && nav.smart[dev].ready) return nav.smart[dev].data;
    if (nav.smart[dev] && nav.smart[dev].pending) return nav.smart[dev].pending;
    const pending = j(smartURL(dev))
      .catch(function (e) {
        return { device: dev, error: e.message || String(e), json: null };
      })
      .then(function (data) {
        nav.smart[dev] = { ready: true, data: data };
        return data;
      });
    nav.smart[dev] = { pending: pending };
    return pending;
  }

  async function renderSidebar() {
    const r = parseRoute();
    const active = routeActive(r);
    let pools = [];
    let disks = [];
    try {
      pools = await getPools();
      disks = await getDisks();
    } catch (e) {
      sidebarEl.innerHTML = '<p class="nav-empty err">' + esc(e.message || e) + '</p>';
      return;
    }

    const key =
      pools
        .map(function (p) {
          return p.name;
        })
        .join(',') +
      '|' +
      disks
        .map(function (d) {
          return d.device;
        })
        .join(',') +
      '|' +
      JSON.stringify(active) +
      '|' +
      nav.poolsOpen +
      '|' +
      nav.disksOpen +
      '|' +
      JSON.stringify(nav.poolKids) +
      '|' +
      JSON.stringify(nav.diskKids) +
      '|' +
      Object.keys(nav.smart)
        .filter(function (k) {
          return nav.smart[k] && nav.smart[k].ready;
        })
        .join(',');
    if (key === sidebarBuiltFor && sidebarEl.querySelector('.nav-section')) {
      // still refresh active classes cheaply by rebuilding — fine for density
    }
    sidebarBuiltFor = key;

    let html =
      '<div class="nav-static">' +
      '<a class="nav-link' +
      (active.type === 'home' ? ' active' : '') +
      '" href="#/">Overview</a>' +
      '<a class="nav-link' +
      (active.type === 'host' ? ' active' : '') +
      '" href="#/host">Host</a>' +
      '<a class="nav-link' +
      (active.type === 'remote' ? ' active' : '') +
      '" href="#/remote">Remote</a>' +
      '</div>';

    html +=
      '<div class="nav-section">' +
      '<button type="button" class="nav-section-head" data-toggle="pools">' +
      '<span class="nav-chevron">' +
      (nav.poolsOpen ? '▾' : '▸') +
      '</span> Pools <span class="muted">(' +
      pools.length +
      ')</span></button>';
    if (nav.poolsOpen) {
      html += '<div class="nav-section-body">';
      if (!pools.length) {
        html += '<div class="nav-empty">No pools</div>';
      } else {
        pools.forEach(function (p) {
          const open = !!nav.poolKids[p.name];
          const isActive = active.type === 'pool' && active.id === p.name;
          html +=
            '<div class="nav-item-row">' +
            '<button type="button" class="nav-twist" data-pool-kid="' +
            esc(p.name) +
            '">' +
            (open ? '▾' : '▸') +
            '</button>' +
            '<a class="nav-item' +
            (isActive ? ' active' : '') +
            '" href="' +
            esc(poolHref(p.name)) +
            '" title="' +
            esc(p.health) +
            '">' +
            esc(p.name) +
            ' <span class="tag tag-health">' +
            esc(p.health) +
            '</span></a></div>';
          if (open) {
            html += '<div class="nav-kids" data-pool-kids-for="' + esc(p.name) + '">';
            html += '<span class="muted small">Loading…</span></div>';
          }
        });
      }
      html += '</div>';
    }
    html += '</div>';

    html +=
      '<div class="nav-section">' +
      '<button type="button" class="nav-section-head" data-toggle="disks">' +
      '<span class="nav-chevron">' +
      (nav.disksOpen ? '▾' : '▸') +
      '</span> Disks <span class="muted">(' +
      disks.length +
      ')</span></button>';
    if (nav.disksOpen) {
      html += '<div class="nav-section-body">';
      if (!disks.length) {
        html += '<div class="nav-empty">No disks in pool configs</div>';
      } else {
        disks.forEach(function (d) {
          const open = !!nav.diskKids[d.device];
          const isActive = active.type === 'disk' && active.id === d.device;
          const poolNames = (d.pools || [])
            .map(function (m) {
              return m.pool;
            })
            .join(', ');
          html +=
            '<div class="nav-item-row">' +
            '<button type="button" class="nav-twist" data-disk-kid="' +
            esc(d.device) +
            '">' +
            (open ? '▾' : '▸') +
            '</button>' +
            '<a class="nav-item' +
            (isActive ? ' active' : '') +
            '" href="' +
            esc(diskHref(d.device)) +
            '" title="' +
            esc(d.device + (poolNames ? ' · ' + poolNames : '')) +
            '">' +
            esc(shortDev(d.device)) +
            '</a></div>';
          if (open) {
            html += '<div class="nav-kids" data-disk-kids-for="' + esc(d.device) + '">';
            const sm = nav.smart[d.device] && nav.smart[d.device].ready ? nav.smart[d.device].data : null;
            html +=
              '<div class="smart-snip">' +
              (sm ? smartSnippetHtml(sm) : '<span class="muted">Loading SMART…</span>') +
              '</div>';
            (d.pools || []).forEach(function (m) {
              html +=
                '<a href="' +
                esc(poolHref(m.pool)) +
                '">' +
                esc(m.pool) +
                (m.state ? ' · ' + esc(m.state) : '') +
                '</a>';
            });
            html +=
              '<a href="' + esc(diskHref(d.device)) + '">Full SMART →</a></div>';
          }
        });
      }
      html += '</div>';
    }
    html += '</div>';

    sidebarEl.innerHTML = html;
    wireSidebarClicks();

    // Lazy-fill pool kids and disk SMART
    Object.keys(nav.poolKids).forEach(function (pname) {
      if (!nav.poolKids[pname]) return;
      fillPoolKids(pname);
    });
    Object.keys(nav.diskKids).forEach(function (dev) {
      if (!nav.diskKids[dev]) return;
      ensureDiskSmart(dev).then(function () {
        const box = sidebarEl.querySelector(
          '.nav-kids[data-disk-kids-for="' + cssAttrEscape(dev) + '"] .smart-snip'
        );
        if (box && nav.smart[dev] && nav.smart[dev].ready) {
          box.innerHTML = smartSnippetHtml(nav.smart[dev].data);
        }
      });
    });
  }

  async function fillPoolKids(pname) {
    const box = sidebarEl.querySelector(
      '.nav-kids[data-pool-kids-for="' + cssAttrEscape(pname) + '"]'
    );
    if (!box) return;
    try {
      const [st, datasets] = await Promise.all([
        j('/v1/pools/' + encSeg(pname) + '/status'),
        j('/v1/datasets?pool=' + encSeg(pname)),
      ]);
      const enriched = enrichConfig(st.config || [], pname);
      const diskLines = enriched.filter(function (l) {
        return l.isDisk;
      });
      const fs = datasets.filter(function (d) {
        return d.type === 'filesystem';
      });
      const vols = datasets.filter(function (d) {
        return d.type === 'volume';
      });
      let html = '';
      diskLines.slice(0, 40).forEach(function (l) {
        html +=
          '<a href="' +
          esc(diskHref(l.name)) +
          '">disk ' +
          esc(shortDev(l.name)) +
          '</a>';
      });
      fs.slice(0, 30).forEach(function (d) {
        html += '<a href="' + esc(datasetHref(pname, d.name)) + '">' + esc(d.name) + '</a>';
      });
      vols.slice(0, 20).forEach(function (d) {
        html += '<a href="' + esc(zvolHref(pname, d.name)) + '">zvol ' + esc(d.name) + '</a>';
      });
      if (!html) html = '<span class="muted">Empty</span>';
      box.innerHTML = html;
    } catch (e) {
      box.innerHTML = '<span class="err">' + esc(e.message || e) + '</span>';
    }
  }

  function wireSidebarClicks() {
    sidebarEl.querySelectorAll('[data-toggle="pools"]').forEach(function (btn) {
      btn.onclick = function () {
        nav.poolsOpen = !nav.poolsOpen;
        renderSidebar();
      };
    });
    sidebarEl.querySelectorAll('[data-toggle="disks"]').forEach(function (btn) {
      btn.onclick = function () {
        nav.disksOpen = !nav.disksOpen;
        renderSidebar();
      };
    });
    sidebarEl.querySelectorAll('[data-pool-kid]').forEach(function (btn) {
      btn.onclick = function (e) {
        e.preventDefault();
        e.stopPropagation();
        const name = btn.getAttribute('data-pool-kid');
        nav.poolKids[name] = !nav.poolKids[name];
        renderSidebar();
      };
    });
    sidebarEl.querySelectorAll('[data-disk-kid]').forEach(function (btn) {
      btn.onclick = function (e) {
        e.preventDefault();
        e.stopPropagation();
        const dev = btn.getAttribute('data-disk-kid');
        nav.diskKids[dev] = !nav.diskKids[dev];
        renderSidebar();
      };
    });
  }

  async function renderHome() {
    renderBreadcrumbs(null);
    appEl.innerHTML = '<p class="loading">Loading…</p>';
    try {
      const [pools, disks, host] = await Promise.all([getPools(), getDisks(), getHost()]);
      const poolRows = pools
        .map(function (p) {
          const usedPct = p.size > 0 ? Math.round((100 * p.allocated) / p.size) : 0;
          return (
            '<tr><td>' +
            poolLink(p.name) +
            '</td><td><span class="tag tag-health">' +
            esc(p.health) +
            '</span></td><td class="mono">' +
            fmtBytes(p.allocated) +
            '</td><td class="mono">' +
            fmtBytes(p.size) +
            '</td><td class="mono">' +
            usedPct +
            '%</td></tr>'
          );
        })
        .join('');
      const diskRows = disks
        .map(function (d) {
          const poolsHtml = (d.pools || [])
            .map(function (m) {
              return poolLink(m.pool);
            })
            .join(', ');
          return (
            '<tr><td>' +
            diskLink(d.device) +
            '</td><td class="mono small">' +
            esc(d.device) +
            '</td><td>' +
            (poolsHtml || '—') +
            '</td></tr>'
          );
        })
        .join('');
      appEl.innerHTML =
        '<h2 class="page-title">Overview</h2>' +
        '<p class="lede">Dense inventory of pools and disks. Use the left nav to keep both lists open; ' +
        'links jump between matching resources.</p>' +
        '<div class="panel">' +
        '<div class="stat-row">' +
        '<span><span class="k">Host</span><a href="#/host">' +
        esc(host.hostname || '—') +
        '</a></span>' +
        '<span><span class="k">Pools</span>' +
        pools.length +
        '</span>' +
        '<span><span class="k">Disks</span>' +
        disks.length +
        '</span>' +
        '<span><span class="k">Uptime</span>' +
        esc(fmtUptime(host.uptimeSeconds)) +
        '</span>' +
        '</div></div>' +
        '<h3 class="sub">Pools</h3>' +
        '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Health</th><th>Used</th><th>Size</th><th>%</th></tr></thead><tbody>' +
        (poolRows || '<tr><td colspan="5" class="muted">None</td></tr>') +
        '</tbody></table></div>' +
        '<h3 class="sub">Disks</h3>' +
        '<div class="table-wrap"><table><thead><tr><th>Disk</th><th>Path</th><th>Pools</th></tr></thead><tbody>' +
        (diskRows || '<tr><td colspan="3" class="muted">None</td></tr>') +
        '</tbody></table></div>';
    } catch (e) {
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  async function renderHost() {
    renderBreadcrumbs([
      { label: 'Overview', hash: '/' },
      { label: 'Host', hash: null },
    ]);
    appEl.innerHTML = '<p class="loading">Loading host…</p>';
    try {
      const tab = (parseRoute().query && parseRoute().query.tab) || 'info';
      const h = await getHost(true);
      const [mod, klog] = await Promise.all([
        tab === 'module' ? j('/v1/module-params') : Promise.resolve(null),
        tab === 'klog' ? j('/v1/kernel-log') : Promise.resolve(null),
      ]);
      const os = [h.osName, h.osVersion].filter(Boolean).join(' ');
      let body = '';
      if (tab === 'info') {
        let arcBlock = '';
        if (h.arc && Object.keys(h.arc).length) {
          const rows = Object.keys(h.arc)
            .sort()
            .map(function (k) {
              return '<tr><th>' + esc(k) + '</th><td class="mono">' + fmtARCval(h.arc[k]) + '</td></tr>';
            })
            .join('');
          arcBlock = '<h3 class="sub">ARC</h3><div class="table-wrap"><table>' + rows + '</table></div>';
        }
        const zfs =
          h.zfsVersions && h.zfsVersions.length
            ? h.zfsVersions.map(esc).join('<br>')
            : '—';
        const mem =
          h.memTotalKb > 0
            ? fmtBytes(h.memTotalKb * 1024) + ' · ' + fmtBytes(h.memAvailableKb * 1024) + ' avail'
            : '—';
        body =
          '<div class="panel">' +
          renderKV(
            [
              ['Hostname', h.hostname],
              ['OS', os || '—'],
              ['Kernel', h.kernel || '—'],
              ['Arch', h.arch || '—'],
              ['Uptime', fmtUptime(h.uptimeSeconds)],
              ['Memory', mem],
              ['ZFS', ''],
              ['Agent', h.agentVersion || '—'],
              ['Collected', h.collectedAt || '—'],
            ],
            false
          ).replace(
            '<dt>ZFS</dt><dd></dd>',
            '<dt>ZFS</dt><dd>' +
              zfs +
              (h.zfsMismatch ? '<br><span class="err">user/kernel mismatch</span>' : '') +
              '</dd>'
          ) +
          '</div>' +
          arcBlock;
      } else if (tab === 'module') {
        const keys = Object.keys(mod || {}).sort();
        body =
          '<div class="table-wrap"><table><thead><tr><th>Param</th><th>Value</th></tr></thead><tbody>' +
          (keys.length
            ? keys
                .map(function (k) {
                  return (
                    '<tr><td class="mono">' +
                    esc(k) +
                    '</td><td class="mono">' +
                    esc(mod[k]) +
                    '</td></tr>'
                  );
                })
                .join('')
            : '<tr><td colspan="2" class="muted">No /sys/module/zfs/parameters</td></tr>') +
          '</tbody></table></div>';
      } else {
        const lines = Array.isArray(klog) ? klog : [];
        body =
          '<pre>' +
          (lines.length ? esc(lines.join('\n')) : 'No matching kernel log lines.') +
          '</pre>';
      }
      appEl.innerHTML =
        '<h2 class="page-title">Host</h2>' +
        tabsHtml(
          [
            ['info', 'Info'],
            ['module', 'Module params'],
            ['klog', 'Kernel log'],
          ],
          tab,
          '#/host'
        ) +
        body;
    } catch (e) {
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  function tabsHtml(tabs, active, baseHash) {
    return (
      '<div class="tabs">' +
      tabs
        .map(function (t) {
          const h = baseHash + '?tab=' + encSeg(t[0]);
          return (
            '<a class="tab' +
            (active === t[0] ? ' active' : '') +
            '" href="' +
            esc(h) +
            '">' +
            esc(t[1]) +
            '</a>'
          );
        })
        .join('') +
      '</div>'
    );
  }

  async function renderRemote() {
    renderBreadcrumbs([
      { label: 'Overview', hash: '/' },
      { label: 'Remote', hash: null },
    ]);
    const gh = (document.body.getAttribute('data-github-repo') || '').trim();
    appEl.innerHTML =
      '<h2 class="page-title">Remote access</h2>' +
      '<p class="lede">Point the UI at an agent on another host over SSH, or serve the bundled UI with auth.</p>' +
      '<div class="panel">' +
      '<h3 class="sub" style="margin-top:0">SSH port forward</h3>' +
      '<pre>ssh -L 8787:127.0.0.1:8787 user@zfs-host\n' +
      '# on the remote host:\nzfstool agent -socket /run/zfstool/agent.sock -http 127.0.0.1:8787\n' +
      '# or:\nzfstool web -listen 127.0.0.1:8787 -agent-socket /run/zfstool/agent.sock</pre>' +
      '<h3 class="sub">Local GUI → remote agent</h3>' +
      '<pre>zfstool gui -agent-url http://127.0.0.1:8787\nzfstool -agent-socket /run/zfstool/agent.sock</pre>' +
      '<h3 class="sub">Web auth (non-loopback)</h3>' +
      '<pre>export ZFSTOOL_WEB_USER=admin\nexport ZFSTOOL_WEB_PASSWORD=secret\nzfstool web -listen 0.0.0.0:8787 -agent-socket /run/zfstool/agent.sock</pre>' +
      '<h3 class="sub">systemd agent</h3>' +
      '<pre>sudo systemctl enable --now zfstool-agent\n# socket: /run/zfstool/agent.sock</pre>' +
      (gh
        ? '<p class="small muted">Source: <a href="' +
          esc(gh) +
          '" target="_blank" rel="noopener">' +
          esc(gh) +
          '</a></p>'
        : '') +
      '</div>';
  }

  async function renderPool(poolName) {
    const tab = (parseRoute().query && parseRoute().query.tab) || 'status';
    renderBreadcrumbs([
      { label: 'Overview', hash: '/' },
      { label: poolName, hash: null },
    ]);
    appEl.innerHTML = '<p class="loading">Loading pool…</p>';
    try {
      const base = '/v1/pools/' + encSeg(poolName);
      const needs = {
        status: true,
        datasets: tab === 'datasets' || tab === 'status' || tab === 'diff',
        history: tab === 'history',
        maintenance: tab === 'maint',
        properties: tab === 'props',
        iostat: tab === 'io',
        graph: tab === 'graph',
        klog: tab === 'klog',
      };
      const [summary, st, datasets, history, maint, props, iostat, graph, klog, bookmarks] =
        await Promise.all([
          getPools().then(function (ps) {
            return ps.find(function (x) {
              return x.name === poolName;
            });
          }),
          j(base + '/status'),
          needs.datasets ? j('/v1/datasets?pool=' + encSeg(poolName)) : Promise.resolve([]),
          needs.history ? j(base + '/history?limit=200') : Promise.resolve([]),
          needs.maintenance ? j(base + '/maintenance') : Promise.resolve(null),
          needs.properties ? j(base + '/properties') : Promise.resolve(null),
          needs.iostat ? j('/v1/iostat') : Promise.resolve([]),
          needs.graph ? j('/v1/graph?pool=' + encSeg(poolName)) : Promise.resolve(null),
          needs.klog ? j('/v1/kernel-log?pool=' + encSeg(poolName)) : Promise.resolve([]),
          tab === 'datasets'
            ? j('/v1/bookmarks?pool=' + encSeg(poolName)).catch(function () {
                return [];
              })
            : Promise.resolve([]),
        ]);

      const enriched = enrichConfig(st.config || [], poolName);
      const sum = summary || {};
      const cap = sum.size > 0 ? Math.round((100 * sum.allocated) / sum.size) : 0;

      let body = '';
      if (tab === 'status') {
        let scanLine = '';
        if (st.scan && st.scan.raw) {
          scanLine = '<p class="scan-line"><strong>Scan</strong> · ' + esc(st.scan.raw) + '</p>';
        } else if (st.scanRaw) {
          scanLine = '<p class="scan-line"><strong>Scan</strong> · ' + esc(st.scanRaw) + '</p>';
        }
        const treeRows = enriched
          .map(function (ln, idx) {
            const pad = Math.min(ln.indent * 0.7, 18);
            let link = esc(ln.name);
            if (ln.isDisk) {
              link = '<a href="' + esc(diskHref(ln.name)) + '">' + esc(ln.name) + '</a>';
            } else if (ln.name !== poolName) {
              link = '<a href="' + esc(vdevHref(poolName, idx)) + '">' + esc(ln.name) + '</a>';
            } else {
              link = poolLink(poolName);
            }
            return (
              '<tr><td class="tree-name" style="padding-left:' +
              pad +
              'rem">' +
              link +
              ' <span class="tag tag-role">' +
              esc(ln.role) +
              '</span></td><td>' +
              esc(ln.state || '—') +
              '</td><td class="mono">' +
              esc(ln.read || '—') +
              '</td><td class="mono">' +
              esc(ln.write || '—') +
              '</td><td class="mono">' +
              esc(ln.cksum || '—') +
              '</td></tr>'
            );
          })
          .join('');
        body =
          '<div class="panel"><div class="stat-row">' +
          '<span><span class="k">State</span>' +
          esc(st.state) +
          '</span>' +
          '<span><span class="k">Health</span><span class="tag tag-health">' +
          esc(sum.health || '—') +
          '</span></span>' +
          '<span><span class="k">Size</span>' +
          fmtBytes(sum.size) +
          '</span>' +
          '<span><span class="k">Alloc</span>' +
          fmtBytes(sum.allocated) +
          ' (' +
          cap +
          '%)</span>' +
          '<span><span class="k">Free</span>' +
          fmtBytes(sum.free) +
          '</span>' +
          (st.errors
            ? '<span class="err"><span class="k">Errors</span>' + esc(st.errors) + '</span>'
            : '') +
          '</div>' +
          scanLine +
          '</div>' +
          '<div class="table-wrap"><table class="tree-table"><thead><tr><th>Name</th><th>State</th><th>R</th><th>W</th><th>C</th></tr></thead><tbody>' +
          treeRows +
          '</tbody></table></div>';
      } else if (tab === 'datasets') {
        const filesystems = datasets.filter(function (d) {
          return d.type === 'filesystem';
        });
        const volumes = datasets.filter(function (d) {
          return d.type === 'volume';
        });
        const snapshots = datasets.filter(function (d) {
          return d.type === 'snapshot';
        });
        body =
          '<h3 class="sub">Filesystems (' +
          filesystems.length +
          ')</h3>' +
          '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Used</th><th>Avail</th><th>Mount</th></tr></thead><tbody>' +
          (filesystems
            .map(function (d) {
              return (
                '<tr><td>' +
                dsLink(d.name, 'filesystem') +
                '</td><td class="mono">' +
                fmtBytes(d.used) +
                '</td><td class="mono">' +
                fmtBytes(d.avail) +
                '</td><td class="mono">' +
                esc(d.mountpoint || '—') +
                '</td></tr>'
              );
            })
            .join('') || '<tr><td colspan="4" class="muted">None</td></tr>') +
          '</tbody></table></div>' +
          '<h3 class="sub">Volumes (' +
          volumes.length +
          ')</h3>' +
          '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Used</th><th>Refer</th></tr></thead><tbody>' +
          (volumes
            .map(function (d) {
              return (
                '<tr><td>' +
                dsLink(d.name, 'volume') +
                '</td><td class="mono">' +
                fmtBytes(d.used) +
                '</td><td class="mono">' +
                fmtBytes(d.refer) +
                '</td></tr>'
              );
            })
            .join('') || '<tr><td colspan="3" class="muted">None</td></tr>') +
          '</tbody></table></div>' +
          '<h3 class="sub">Snapshots (' +
          snapshots.length +
          ')</h3>' +
          '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Used</th><th>Refer</th></tr></thead><tbody>' +
          (snapshots.length
            ? snapshots
                .slice(0, 100)
                .map(function (d) {
                  return (
                    '<tr><td>' +
                    dsLink(d.name, 'snapshot') +
                    '</td><td class="mono">' +
                    fmtBytes(d.used) +
                    '</td><td class="mono">' +
                    fmtBytes(d.refer) +
                    '</td></tr>'
                  );
                })
                .join('') +
              (snapshots.length > 100
                ? '<tr><td colspan="3" class="muted">… ' +
                  (snapshots.length - 100) +
                  ' more</td></tr>'
                : '')
            : '<tr><td colspan="3" class="muted">None</td></tr>') +
          '</tbody></table></div>' +
          '<h3 class="sub">Bookmarks (' +
          (bookmarks || []).length +
          ')</h3>' +
          '<div class="table-wrap"><table><thead><tr><th>Name</th></tr></thead><tbody>' +
          ((bookmarks || []).length
            ? bookmarks
                .map(function (b) {
                  const n = b.name || b;
                  return '<tr><td class="mono">' + esc(n) + '</td></tr>';
                })
                .join('')
            : '<tr><td class="muted">None</td></tr>') +
          '</tbody></table></div>';
      } else if (tab === 'history') {
        body =
          '<div class="table-wrap"><table><thead><tr><th>Time</th><th>Command</th></tr></thead><tbody>' +
          ((history || [])
            .map(function (e) {
              return (
                '<tr><td class="mono small">' +
                esc(e.time) +
                '</td><td class="mono">' +
                esc(e.command) +
                '</td></tr>'
              );
            })
            .join('') || '<tr><td colspan="2" class="muted">None</td></tr>') +
          '</tbody></table></div>';
      } else if (tab === 'maint') {
        const mb = maint || {};
        const propRows = Object.keys(mb.props || {})
          .sort()
          .map(function (k) {
            return [k, mb.props[k]];
          });
        body =
          '<div class="panel">' +
          renderKV(
            [
              ['Pool', mb.pool || poolName],
              ['Autotrim', mb.autotrim || '—'],
              [
                'Scan',
                mb.scan && mb.scan.raw ? mb.scan.raw : mb.scan ? JSON.stringify(mb.scan) : '—',
              ],
            ],
            false
          ) +
          '</div>' +
          (propRows.length
            ? '<h3 class="sub">Properties</h3>' + renderKV(propRows)
            : '');
      } else if (tab === 'props') {
        const pmap = (props && props.properties) || {};
        const rows = Object.keys(pmap)
          .sort()
          .map(function (k) {
            return [k, pmap[k]];
          });
        body = '<div class="panel">' + renderKV(rows) + '</div>';
      } else if (tab === 'io') {
        const samples = (iostat || []).filter(function (s) {
          return !s.pool || s.pool === poolName || s.pool === 'pool';
        });
        const use = samples.length ? samples : iostat || [];
        body =
          '<div class="table-wrap"><table><thead><tr><th>Pool</th><th>Vdev</th><th>Raw</th></tr></thead><tbody>' +
          (use.length
            ? use
                .map(function (s) {
                  return (
                    '<tr><td>' +
                    (s.pool ? poolLink(s.pool) : '—') +
                    '</td><td>' +
                    esc(s.vdev || '—') +
                    '</td><td class="mono small">' +
                    esc((s.raw || []).join(' ')) +
                    '</td></tr>'
                  );
                })
                .join('')
            : '<tr><td colspan="3" class="muted">No iostat samples</td></tr>') +
          '</tbody></table></div>';
      } else if (tab === 'graph') {
        const list = Array.isArray(graph) ? graph : [];
        body =
          '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Type</th><th>Origin</th><th>Children</th></tr></thead><tbody>' +
          (list.length
            ? list
                .map(function (n) {
                  return (
                    '<tr><td>' +
                    dsLink(n.name, n.type) +
                    '</td><td>' +
                    esc(n.type) +
                    '</td><td>' +
                    (n.origin ? dsLink(n.origin) : '—') +
                    '</td><td class="small">' +
                    (n.children || [])
                      .map(function (c) {
                        return dsLink(c);
                      })
                      .join(', ') +
                    '</td></tr>'
                  );
                })
                .join('')
            : '<tr><td colspan="4" class="muted">No graph nodes</td></tr>') +
          '</tbody></table></div>';
      } else if (tab === 'klog') {
        body =
          '<pre>' +
          (Array.isArray(klog) && klog.length ? esc(klog.join('\n')) : 'No matching lines.') +
          '</pre>';
      } else if (tab === 'diff') {
        body =
          '<div class="panel">' +
          '<p class="muted small">POST /v1/zfs-diff — compare two snapshots or a snapshot and a live dataset.</p>' +
          '<div class="form-row">' +
          '<input id="diff-from" placeholder="pool/ds@snap1" />' +
          '<input id="diff-to" placeholder="pool/ds@snap2 or pool/ds" />' +
          '<button type="button" class="btn primary" id="diff-run">Diff</button>' +
          '</div><pre id="diff-out" hidden></pre></div>';
      }

      appEl.innerHTML =
        '<h2 class="page-title">' +
        esc(poolName) +
        '</h2>' +
        tabsHtml(
          [
            ['status', 'Status'],
            ['datasets', 'Datasets'],
            ['history', 'History'],
            ['maint', 'Maintenance'],
            ['props', 'Properties'],
            ['io', 'I/O'],
            ['graph', 'Graph'],
            ['klog', 'Kernel log'],
            ['diff', 'Diff'],
          ],
          tab,
          '#/pool/' + encSeg(poolName)
        ) +
        body;

      if (tab === 'diff') {
        const btn = document.getElementById('diff-run');
        if (btn) {
          btn.onclick = async function () {
            const from = document.getElementById('diff-from').value.trim();
            const to = document.getElementById('diff-to').value.trim();
            const out = document.getElementById('diff-out');
            out.hidden = false;
            out.textContent = 'Running…';
            try {
              const res = await j('/v1/zfs-diff', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ from: from, to: to }),
              });
              out.textContent = res.output || '(empty)';
            } catch (err) {
              out.textContent = err.message || String(err);
            }
          };
        }
      }
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
        { label: 'Overview', hash: '/' },
        { label: poolName, hash: '/pool/' + encSeg(poolName) },
        { label: ln.name, hash: null },
      ]);
      const childRows = kids
        .map(function (k) {
          const c = k.line;
          const nm = c.isDisk
            ? '<a href="' + esc(diskHref(c.name)) + '">' + esc(c.name) + '</a>'
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
        '<h2 class="page-title">' +
        esc(ln.name) +
        ' <span class="tag tag-role">' +
        esc(ln.role) +
        '</span></h2>' +
        '<div class="panel">' +
        renderKV(
          [
            ['Pool', ''],
            ['Under', ln.vdevPath],
            ['State', ln.state || '—'],
            ['READ', ln.read || '—'],
            ['WRITE', ln.write || '—'],
            ['CKSUM', ln.cksum || '—'],
          ],
          false
        ).replace('<dt>Pool</dt><dd></dd>', '<dt>Pool</dt><dd>' + poolLink(poolName) + '</dd>') +
        '</div>' +
        '<h3 class="sub">Children</h3>' +
        '<div class="table-wrap"><table><thead><tr><th>Name</th><th>Role</th><th>State</th></tr></thead><tbody>' +
        (childRows || '<tr><td colspan="3" class="muted">None</td></tr>') +
        '</tbody></table></div>';
    } catch (e) {
      renderBreadcrumbs([
        { label: 'Overview', hash: '/' },
        { label: poolName, hash: '/pool/' + encSeg(poolName) },
        { label: 'Vdev', hash: null },
      ]);
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  async function renderDisk(devPath) {
    renderBreadcrumbs([
      { label: 'Overview', hash: '/' },
      { label: shortDev(devPath), hash: null },
    ]);
    appEl.innerHTML = '<p class="loading">Loading disk…</p>';
    try {
      const disks = await getDisks();
      const hit = disks.find(function (d) {
        return d.device === devPath;
      });
      const smart = await ensureDiskSmart(devPath);
      const sj = smart && smart.json;
      const hasJson = sj && typeof sj === 'object' && Object.keys(sj).length > 0;

      let membership = '';
      const members = (hit && hit.pools) || [];
      if (members.length) {
        membership =
          '<h3 class="sub">In pools</h3><div class="table-wrap"><table><thead><tr><th>Pool</th><th>State</th><th>Path</th></tr></thead><tbody>' +
          members
            .map(function (m) {
              return (
                '<tr><td>' +
                poolLink(m.pool) +
                '</td><td>' +
                esc(m.state || '—') +
                '</td><td class="mono small">' +
                esc(m.path || '—') +
                '</td></tr>'
              );
            })
            .join('') +
          '</tbody></table></div>';
      } else {
        membership = '<p class="muted">Not found in current pool configs.</p>';
      }

      let smartBlock = '';
      if (hasJson) {
        smartBlock =
          '<h3 class="sub">SMART</h3><div class="panel">' +
          (smart.error ? '<p class="err small">' + esc(smart.error) + '</p>' : '') +
          renderKV(summarizeSmart(sj)) +
          '<details class="raw"><summary>Raw JSON</summary><pre>' +
          esc(JSON.stringify(smart, null, 2)) +
          '</pre></details></div>';
      } else if (smart && smart.error) {
        smartBlock =
          '<h3 class="sub">SMART</h3><div class="panel"><p class="err">' +
          esc(smart.error) +
          '</p>' +
          smartHintHtml(smart) +
          '</div>';
      } else {
        smartBlock = '<h3 class="sub">SMART</h3><p class="muted">No SMART data.</p>';
      }

      appEl.innerHTML =
        '<h2 class="page-title">' +
        esc(shortDev(devPath)) +
        '</h2>' +
        '<div class="panel">' +
        renderKV(
          [
            ['Device', ''],
            ['Path', devPath],
          ],
          false
        ).replace(
          '<dt>Device</dt><dd></dd>',
          '<dt>Device</dt><dd><a href="' + esc(diskHref(devPath)) + '">' + esc(devPath) + '</a></dd>'
        ) +
        '</div>' +
        membership +
        smartBlock;

      // Keep sidebar disk expanded when viewing
      nav.diskKids[devPath] = true;
      nav.disksOpen = true;
    } catch (e) {
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  async function renderDatasetLike(kind, poolName, dsName) {
    const label = kind === 'zvol' ? 'Zvol' : 'Dataset';
    renderBreadcrumbs([
      { label: 'Overview', hash: '/' },
      { label: poolName, hash: '/pool/' + encSeg(poolName) },
      { label: dsName, hash: null },
    ]);
    appEl.innerHTML = '<p class="loading">Loading…</p>';
    try {
      const data = await j('/v1/datasets/properties?name=' + encSeg(dsName));
      const props = data.properties || {};
      const isSnap = (props.type || kind) === 'snapshot' || dsName.indexOf('@') >= 0;
      const [holds, allow] = await Promise.all([
        isSnap
          ? j('/v1/snapshots/holds?snapshot=' + encSeg(dsName)).catch(function () {
              return null;
            })
          : Promise.resolve(null),
        j('/v1/zfs-allow?dataset=' + encSeg(dsName.split('@')[0])).catch(function () {
          return null;
        }),
      ]);

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
        if (props[k] == null) continue;
        let val = esc(props[k]);
        if (k === 'origin' && props[k]) val = dsLink(props[k]);
        if (k === 'mountpoint' && props[k] && props[k].charAt(0) === '/') {
          val = '<span class="mono">' + esc(props[k]) + '</span>';
        }
        priority.push([k, val]);
      }
      const rest = Object.keys(props)
        .filter(function (k) {
          return interesting.indexOf(k) < 0;
        })
        .sort()
        .map(function (k) {
          return [k, esc(props[k])];
        });

      let holdsBlock = '';
      if (isSnap) {
        const holdMap = holds && typeof holds === 'object' && !Array.isArray(holds) ? holds : {};
        const tags = Object.keys(holdMap);
        holdsBlock =
          '<h3 class="sub">Holds</h3><div class="panel">' +
          (tags.length
            ? '<div class="table-wrap"><table><thead><tr><th>Tag</th><th>Info</th></tr></thead><tbody>' +
              tags
                .map(function (t) {
                  return (
                    '<tr><td class="mono">' +
                    esc(t) +
                    '</td><td class="mono">' +
                    esc(holdMap[t]) +
                    '</td></tr>'
                  );
                })
                .join('') +
              '</tbody></table></div>'
            : '<span class="muted">None</span>') +
          '</div>';
      }

      const allowOut = allow && allow.output != null ? allow.output : '';
      const allowBlock =
        '<h3 class="sub">zfs allow</h3><div class="panel"><pre>' +
        (allowOut ? esc(allowOut) : '—') +
        '</pre></div>';

      const parentFs = dsName.indexOf('@') >= 0 ? dsName.split('@')[0] : dsName.replace(/\/[^/]+$/, '');

      appEl.innerHTML =
        '<h2 class="page-title">' +
        esc(dsName) +
        '</h2>' +
        '<div class="panel"><div class="stat-row">' +
        '<span><span class="k">Pool</span>' +
        poolLink(poolName) +
        '</span>' +
        '<span><span class="k">Type</span><span class="tag">' +
        esc(props.type || kind) +
        '</span></span>' +
        (parentFs && parentFs !== dsName
          ? '<span><span class="k">Parent</span>' + dsLink(parentFs) + '</span>'
          : '') +
        (props.origin ? '<span><span class="k">Origin</span>' + dsLink(props.origin) + '</span>' : '') +
        '</div></div>' +
        '<div class="panel">' +
        renderKV(priority.concat(rest), true) +
        '</div>' +
        holdsBlock +
        allowBlock;
    } catch (e) {
      appEl.innerHTML = '<p class="err">' + esc(e.message || e) + '</p>';
    }
  }

  async function navigateRoute(r) {
    switch (r.kind) {
      case 'home':
        await renderHome();
        break;
      case 'host':
        await renderHost();
        break;
      case 'remote':
        await renderRemote();
        break;
      case 'pool':
        await renderPool(r.parts[0]);
        break;
      case 'vdev':
        await renderVdev(r.parts[0], r.parts[1]);
        break;
      case 'disk':
        await renderDisk(r.parts[0]);
        break;
      case 'dataset':
        await renderDatasetLike('dataset', r.parts[0], r.parts[1]);
        break;
      case 'zvol':
        await renderDatasetLike('zvol', r.parts[0], r.parts[1]);
        break;
      default:
        await renderHome();
    }
    enhanceFilterableLists(appEl);
    await renderSidebar();
    await updateConnectionChrome();
  }

  function dispatch() {
    navigateRoute(parseRoute()).catch(function (err) {
      if (typeof console !== 'undefined' && console.error) console.error(err);
    });
  }

  async function refreshCurrentView() {
    var x =
      window.scrollX != null
        ? window.scrollX
        : document.documentElement.scrollLeft || document.body.scrollLeft || 0;
    var y =
      window.scrollY != null
        ? window.scrollY
        : document.documentElement.scrollTop || document.body.scrollTop || 0;
    poolsCache = null;
    disksCache = null;
    hostCache = null;
    nav.smart = {};
    sidebarBuiltFor = '';
    try {
      await navigateRoute(parseRoute());
    } catch (err) {
      if (typeof console !== 'undefined' && console.error) console.error(err);
    }
    function restoreScroll() {
      window.scrollTo(x, y);
    }
    restoreScroll();
    requestAnimationFrame(function () {
      restoreScroll();
      requestAnimationFrame(restoreScroll);
    });
  }

  function invokeNativeExit() {
    if (typeof window.zfstoolExit !== 'function') return;
    try {
      var p = window.zfstoolExit();
      if (p && typeof p.then === 'function') p.catch(function () {});
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
