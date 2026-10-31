/* 派工台前端逻辑：纯原生 JS，不引构建工具。
   调 Go 的写法是 window.go.app.App.方法名(...)，事件用 window.runtime.EventsOn。
   这一层只负责显示与按钮，真正的判断都在 Go 里。 */
'use strict';

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s == null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// Go 那边绑定的入口（包名 app、结构体 App）。
function api() {
  const g = window.go && window.go.app && window.go.app.App;
  if (!g) throw new Error('程序后台尚未就绪');
  return g;
}

let windowsCache = [];
let settingsCache = null;
let logLines = [];
let dbgTab = ''; // --tab debug 时把前端自测信息回写到日志

/* ---------- 小工具 ---------- */

// 时间统一按「毫秒」传：任务库与 OpenClaw 会话库存的都是毫秒（
// 当成秒会把提交时间算到几天后、窗口「多久以前动过」永远显示 0 秒前）。
function fmtTime(ms) {
  if (!ms) return '';
  const d = new Date(ms);
  const p = (n) => String(n).padStart(2, '0');
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

function ago(ms) {
  if (!ms) return '';
  const s = Math.max(0, Math.floor((Date.now() - ms) / 1000));
  if (s < 60) return s + tr(' 秒前');
  if (s < 3600) return Math.floor(s / 60) + tr(' 分钟前');
  if (s < 86400) return Math.floor(s / 3600) + tr(' 小时前');
  return Math.floor(s / 86400) + tr(' 天前');
}

function feed(msg) { $('feed').textContent = tr(msg); }

function chip(el, text, cls) {
  if (!el) return;
  el.textContent = tr(text);
  el.className = 'chip' + (cls ? ' ' + cls : '');
}

// 模型服务那句太长，顶栏只留结论（完整说法在设置页与日志里）。
// 左边比的是后端传来的原文（不能翻），右边才是给用户看的短词。
function serviceShort(s) {
  if (s === '模型服务空闲') return tr('服务空闲');
  if (s === '模型服务繁忙') return tr('服务忙');
  return tr('服务未知');
}

/* 顶栏「距下次检查」：引擎是每隔 N 秒跑一轮（不是每条任务各自倒计时）
   所以这里显示的是全局节拍；本地每秒走一秒，不用每次都问后端。 */
let nextTickSec = -1;
function renderNextChip(running, sec, interval) {
  const el = $('chip-next');
  if (!el) return;
  if (!running) { chip(el, tr('已暂停'), 'warn'); return; }
  chip(el, tr('下次 ') + Math.max(0, sec) + tr(' 秒'), '');
  void interval;
}
setInterval(() => {
  if (nextTickSec < 0) return;
  nextTickSec = Math.max(0, nextTickSec - 1);
  renderNextChip(true, nextTickSec, 0);
  tickQueueAges();
}, 1000);

/* 排队卡片上的「已等 X 秒」：每秒本地走一秒（时刻从 createdAt 算，刷新后仍准）。 */
function tickQueueAges() {
  document.querySelectorAll('#queue .task .who').forEach((who) => {
    const span = who.querySelector('[data-since]');
    if (!span) return;
    const ms = Number(span.dataset.since);
    if (!ms) return;
    span.textContent = tr('已等 ') + dur(Math.floor((Date.now() - ms) / 1000));
  });
}
function dur(s) {
  if (s < 60) return s + tr(' 秒');
  if (s < 3600) return Math.floor(s / 60) + tr(' 分 ') + (s % 60) + tr(' 秒');
  return Math.floor(s / 3600) + tr(' 小时 ') + Math.floor((s % 3600) / 60) + tr(' 分');
}

/* ---------- 顶栏状态 ---------- */

async function refreshOverview() {
  try {
    const o = await api().Overview();
    // 顶栏徽章一律短词
    // 排队 / 执行 / 完成 / 同时 N 窗 / 每 N 秒 / 距下次 N 秒。行话不出现。
    chip($('chip-run'), o.running ? tr('派工中 每 ') + o.interval + tr(' 秒') : tr('已暂停'), o.running ? 'ok' : 'warn');
    chip($('chip-service'), serviceShort(o.service), o.service === '模型服务空闲' ? 'ok' : (o.service === '模型服务繁忙' ? 'warn' : ''));
    nextTickSec = o.running ? o.nextSec : -1;
    renderNextChip(o.running, o.nextSec, o.interval);
    chip($('chip-par'), tr('同时 ') + o.parallel + tr(' 窗'), '');
    chip($('chip-queue'), tr('排队 ') + o.queueLen, o.queueLen ? 'warn' : '');
    chip($('chip-dispatch'), tr('执行 ') + o.dispatchN, o.dispatchN ? 'warn' : '');
    chip($('chip-done'), tr('完成 ') + o.doneN, o.doneN ? 'ok' : '');
    $('btn-toggle').textContent = o.running ? tr('暂停自动派工') : tr('开始自动派工');
    if (o.errorN) chip($('chip-queue'), tr('排队 ') + o.queueLen + tr('（异常 ') + o.errorN + tr('）'), 'bad');
    renderBanner(o);
    refreshErrors();
    refreshConfirm();
    refreshProbe();
  } catch (e) { feed(tr('读状态失败：') + e.message); }
}

/* 横幅：停下来等处理（全异常）优先，其次准备关机。 */
function renderBanner(o) {
  const b = $('banner');
  b.className = 'banner hidden';
  b.innerHTML = '';
  if (o.halt) {
    b.className = 'banner bad';
    b.innerHTML = `<span class="grow">${tr('⛔ 已停止派发：')}${esc(tr(o.halt))}${tr('。修复后点「重试」，或点「继续派发」。')}</span>
      <button id="bn-resume">${tr('继续派发')}</button>`;
    $('bn-resume').onclick = async () => {
      try { await api().ClearHalt(); await refreshOverview(); } catch (e) { feed(tr('继续失败：') + e.message); }
    };
    return;
  }
  if (o.shutdown) {
    b.className = 'banner warn';
    b.innerHTML = `<span class="grow">${tr('🌙 自动关机：')}${esc(tr(o.shutdown))}</span><button id="bn-undo">${tr('取消关机')}</button>`;
    $('bn-undo').onclick = async () => {
      // 先等后台真撤销完再刷新：不 await 的话刷新可能比撤销先到，横幅会多挂几秒才消。
      try {
        await api().CancelShutdown();
      } catch (e) {
        feed(tr('取消失败：') + e.message);
      }
      refreshOverview();
    };
  }
}

/* 异常任务区：带原因 + 「重试 / 删除」。 */
async function refreshErrors() {
  try {
    const es = await api().Errors() || [];
    const box = $('errors');
    box.innerHTML = '';
    if (!es.length) {
      box.innerHTML = `<div class="empty">${tr('没有标异常的任务。')}</div>`;
      return;
    }
    for (const t of es) {
      const div = document.createElement('div');
      div.className = 'task error';
      div.innerHTML = `<div class="head"><div class="who">→ ${esc(t.targetLabel || t.targetKey)} · ${tr('提交 ')}${fmtTime(t.createdAt)}
        <span class="tag bad">${tr('派发失败')}</span>${t.tries ? tr(' · 重试 ') + t.tries + tr(' 次') : ''}</div></div>
        <div class="body">${esc(t.body)}</div>
        <div class="why">${tr('原因：')}${esc(t.note || tr('未记录'))}</div>
        <div class="acts"><button data-act="retry" data-id="${t.id}">${tr('重试')}</button>
          <button data-act="cancel" data-id="${t.id}" class="danger">${tr('删除')}</button>
          <span class="spacer"></span><span class="stamp">${fmtTime(t.doneAt)}</span></div>`;
      box.appendChild(div);
    }
  } catch (e) { /* 异常区读失败不拦其它刷新 */ }
}

$('card-errors').addEventListener('click', async (ev) => {
  const b = ev.target.closest('button[data-act]');
  if (!b) return;
  const id = Number(b.dataset.id);
  try {
    if (b.dataset.act === 'retry') {
      await api().RetryTask(id);
      feed(`T-${String(id).padStart(3, '0')} ` + tr('已重新排队。'));
    } else {
      await api().CancelTask(id);
      feed(`T-${String(id).padStart(3, '0')} ` + tr('已删除。'));
    }
    await Promise.all([refreshOverview(), refreshQueue()]);
  } catch (e) { feed(tr('操作失败：') + e.message); }
});

/* 同模型并发确认：引擎挂起问题 → 界面弹。 */
// 弹窗句子按字段自己拼：窗口名是 OpenClaw 那边的数据（不翻）。
// 中文界面直接用后端原文，跟以前一模一样。
function trConfirm(r) {
  if (window.__i18n && window.__i18n.getLang && window.__i18n.getLang() === 'en') {
    return `The target window "${r.target}" and the running one "${r.running}" share one model service (${r.endpoint}). `
      + 'The second task would queue inside the service and may slow the first one. Dispatch anyway?';
  }
  return r.question;
}
let confirmShown = 0;
async function refreshConfirm() {
  try {
    const r = await api().PendingConfirm();
    if (!r) {
      $('modal').classList.add('hidden');
      confirmShown = 0;
      return;
    }
    if (confirmShown !== r.taskId) {
      confirmShown = r.taskId;
      $('modal-text').textContent = trConfirm(r);
      $('modal').classList.remove('hidden');
    }
  } catch (e) { /* 问不到不拦其它刷新 */ }
}

async function answerConfirm(accept) {
  // 默认每次都提醒：弹窗里不再顺手改「不再提醒」
  // mute 固定为空（不改设置），要关只去设置页。
  try {
    await api().AnswerConfirm(accept, '');
    $('modal').classList.add('hidden');
    confirmShown = 0;
    feed(accept ? tr('已确认并发，将继续派发。') : tr('已暂不派发，任务保留在队列中。'));
    await Promise.all([refreshOverview(), refreshQueue()]);
  } catch (e) { feed(tr('确认失败：') + e.message); }
}
$('modal-yes').addEventListener('click', () => answerConfirm(true));
$('modal-no').addEventListener('click', () => answerConfirm(false));

/* ---------- 派工页 ---------- */

function renderTargetSelect() {
  const sel = $('target');
  const keep = sel.value;
  sel.innerHTML = '';
  const usable = windowsCache.filter((w) => w.included);
  if (!usable.length) {
    const o = document.createElement('option');
    o.textContent = tr('（尚未勾选任何窗口，请先去「窗口」页勾选）');
    o.value = '';
    sel.appendChild(o);
    return;
  }
  for (const w of usable) {
    const o = document.createElement('option');
    o.value = w.key;
    o.textContent = `${w.label}　${w.idle ? tr('空闲') : tr('执行中')}${w.model ? ' · ' + w.model : ''}`;
    sel.appendChild(o);
  }
  if (keep && usable.some((w) => w.key === keep)) sel.value = keep;
}

/* 队列里就地编辑：点「编辑」→ 这条卡片自己变成输入框
   上方「新建任务」里写的字不受影响；改完按「改好了」才参与派发，期间引擎跳过它。
   editCard 存着这张卡片：定时刷新队列时直接把它接回去，不然输入到一半的字会被刷掉。 */
let editId = 0;

/* 队列里就地编辑：点「编辑」→ 这条卡片自己变成输入框
   上方「新建任务」里写的字完全不动；改完按「改好了」才参与派发，期间引擎跳过它、后面的照常派。 */
function editCardEl(t) {
  const div = document.createElement('div');
  div.className = 'task editing';
  const usable = windowsCache.filter((w) => w.included);
  // 原窗口万一不在「已勾选」名单里（窗口被取消勾选/暂时没扫到），也要把它列进去并选中
  // 否则下拉会默认落到第一个窗口，保存时把发送窗口悄悄换掉。
  const optsList = usable.some((w) => w.key === t.targetKey)
    ? usable
    : [{ key: t.targetKey, label: (t.targetLabel || t.targetKey) + '（原窗口）', model: '' }, ...usable];
  const opts = optsList.map((w) => `<option value="${esc(w.key)}">${esc(w.label)}${w.model ? ' · ' + esc(w.model) : ''}</option>`).join('');
  const atts = (t.attachPaths && t.attachPaths.length) ? t.attachPaths : (t.imagePath ? [t.imagePath] : []);
  const attHtml = atts.length
    ? `<div class="att">${atts.map((p) => `<span class="fi" title="${esc(p)}">${esc(extOf(p) || tr('文件'))}</span>`).join('')}<span class="stamp">${tr('附件不变')}</span></div>`
    : '';
  div.innerHTML = `<div class="head"><div class="who"><b class="tid">T-${String(t.id).padStart(3, '0')}</b> · ${tr('正在编辑，先不派发')}</div></div>
    <select class="target ed-target">${opts}</select>
    <textarea class="ed-body" rows="6">${esc(t.body)}</textarea>
    ${attHtml}
    <div class="acts">
      <button class="primary ed-save" title="${tr('存好并回到排队，按 Ctrl+Enter 也一样')}">${tr('改好了，重新排队')}</button>
      <button class="ed-cancel" title="${tr('丢掉刚才的修改，按原样继续排队')}">${tr('不改了')}</button>
      <button class="danger ed-drop" title="${tr('不要这条了')}">${tr('删除这条')}</button>
      <span class="spacer"></span>
      <span class="stamp">${tr('改完前这条不会被派发，后面的任务照常走')}</span>
    </div>`;
  const sel = div.querySelector('.ed-target');
  sel.value = t.targetKey;
  const ta = div.querySelector('.ed-body');
  div.querySelector('.ed-save').addEventListener('click', () => saveEdit(t.id));
  div.querySelector('.ed-cancel').addEventListener('click', async () => {
    try {
      await api().CancelEdit(t.id); // 内容本来就没写进库（按「改好了」才写），这里只把状态推回排队
      feed(`T-${String(t.id).padStart(3, '0')} ` + tr('已放弃修改，按原样排队。'));
    } catch (e) { feed(tr('操作失败：') + e.message); }
    editId = 0;
    await Promise.all([refreshQueue(), refreshOverview()]);
  });
  div.querySelector('.ed-drop').addEventListener('click', async () => {
    try {
      await api().CancelTask(t.id);
      editId = 0;
      feed(`T-${String(t.id).padStart(3, '0')} ` + tr('已删除。'));
      await Promise.all([refreshQueue(), refreshOverview()]);
    } catch (e) { feed(tr('删除失败：') + e.message); }
  });
  ta.addEventListener('keydown', (ev) => {
    if ((ev.ctrlKey || ev.metaKey) && ev.key === 'Enter') { ev.preventDefault(); saveEdit(t.id); }
  });
  return div;
}

async function saveEdit(id) {
  const card = document.querySelector(`#queue .task.editing`);
  if (!card) return;
  const key = card.querySelector('.ed-target').value;
  const body = card.querySelector('.ed-body').value.trim();
  if (!key) { feed(tr('请先选发送窗口。')); return; }
  if (!body) { feed(tr('任务内容不能空。')); return; }
  try {
    await api().SaveEdit(id, key, body);
    editId = 0;
    feed(`T-${String(id).padStart(3, '0')} ` + tr('已改好，重新排队。'));
    await Promise.all([refreshQueue(), refreshOverview()]);
  } catch (e) { feed(tr('保存失败：') + e.message); }
}

function taskCard(t, kind) {
  const div = document.createElement('div');
  const cls = kind === 'done' ? 'done'
    : kind === 'dispatched'
      ? (t.ack ? 'done' : 'dispatched')
      : (t.state === 'error' ? 'error' : '');
  div.className = 'task ' + kind + (cls ? ' ' + cls : '');

  const head = document.createElement('div');
  head.className = 'head';
  const stateTag = t.state === 'queued'
    ? `<span class="tag">${tr('排队第 ')}${t.prio || ''}</span>`
    : t.state === 'editing'
      ? `<span class="tag warn">${tr('正在编辑·先不派发')}</span>`
      : `<span class="tag ${t.state === 'done' ? 'ok' : (t.state === 'error' ? 'bad' : 'warn')}">${tr(stateText(t))}</span>`;
  // 排队中的任务显示「已等 X 秒」（每秒本地走一秒，不用问后端）。
  const waited = t.state === 'queued' && t.createdAt
    ? `<span class="tag" data-since="${t.createdAt}">${tr('已等 ')}${dur(Math.floor((Date.now() - t.createdAt) / 1000))}</span>`
    : '';
  head.innerHTML = `<div><div class="who"><b class="tid">T-${String(t.id).padStart(3, '0')}</b> → ${esc(t.targetLabel || t.targetKey)} · ${tr('提交 ')}${fmtTime(t.createdAt)} ${stateTag}${waited}${t.tries ? tr(' · 重试 ') + t.tries + tr(' 次') : ''}${t.note ? ' · ' + esc(t.note) : ''}</div></div>`;

  const body = document.createElement('div');
  body.className = 'body';
  body.textContent = t.body;
  // 已派发待回复 / 已完成：任务内容先只显 1 行，点一下展开全文，隔一会儿自动收回。
  if (kind === 'dispatched' || kind === 'done') {
    body.classList.add('clip');
    body.title = tr('点一下看全文，几秒后自动收回');
    body.addEventListener('click', () => {
      body.classList.remove('clip');
      body.classList.add('open');
      clearTimeout(body._fold);
      body._fold = setTimeout(() => body.classList.remove('open'), 25000);
    });
  }
  div.appendChild(head);
  div.appendChild(body);

  // 附件：图片显缩略图（点开放大），其他文件显类型块。
  const atts = (t.attachPaths && t.attachPaths.length) ? t.attachPaths : (t.imagePath ? [t.imagePath] : []);
  if (atts.length) {
    const att = document.createElement('div');
    att.className = 'att';
    att.innerHTML = atts.map((p, i) => {
      const name = p.split(/[\\/]/).pop();
      return isImagePath(p)
        ? `<img data-i="${i}" src="" title="${esc(name)}${tr('（点一下放大）')}" alt="${esc(name)}">`
        : `<span class="fi" data-i="${i}" title="${esc(name)}">${esc(extOf(p) || tr('文件'))}</span>`;
    }).join('');
    div.appendChild(att);
    att.querySelectorAll('img').forEach((im) => {
      const i = Number(im.dataset.i), p = atts[i];
      api().Thumb(p).then((u) => {
        if (u) { im.src = u; return; }
        im.outerHTML = `<span class="fi" data-i="${i}" title="${esc(p.split(/[\\/]/).pop())}">${tr('图')}</span>`;
      }).catch(() => { im.alt = tr('读不到图'); });
    });
    att.addEventListener('click', async (ev) => {
      const cell = ev.target.closest('img, .fi');
      if (!cell) return;
      const p = atts[Number(cell.dataset.i) || 0];
      if (p) await showLightbox(p, await api().FileData(p));
    });
  }

  // 按钮放在卡片底下一行：不会被长内容顶出卡片，也好截图自查。
  const acts = document.createElement('div');
  acts.className = 'acts';
  if (kind === 'queued') {
    if (t.state === 'editing') {
      acts.innerHTML = `<span class="spacer"></span>`;
      div.appendChild(acts);
      return editCardEl(t);
    }
    // 按钮名用「上移 / 下移」。
    acts.innerHTML = `<button data-act="up" data-id="${t.id}">${tr('上移')}</button>
      <button data-act="down" data-id="${t.id}">${tr('下移')}</button>
      <button data-act="cancel" data-id="${t.id}" class="danger">${tr('删除')}</button>
      <span class="spacer"></span>
      <button data-act="copy" data-id="${t.id}">${tr('编辑')}</button>`;
  } else {
    const when = t.doneAt ? tr('完成 ') + fmtTime(t.doneAt) : (t.dispatchedAt ? tr('派发 ') + fmtTime(t.dispatchedAt) : '');
    acts.innerHTML = `<span class="spacer"></span><span class="stamp">${when}</span>`;
  }
  div.appendChild(acts);
  return div;
}

function stateText(t) {
  switch (t.state) {
    case 'dispatched': return t.ack ? '已完成 ✅' : '已派发，等待完成';
    case 'done': return '已完成 ✅';
    case 'done_unacked': return '已完成（未收到回执）';
    case 'error': return '派发失败';
    case 'canceled': return '已删除';
    case 'paused': return '已暂停';
    default: return t.state;
  }
}
async function refreshQueue() {
  try {
    // 正在就地编辑：先记下输入到一半的内容与选的窗口，重建卡片后接回去（不然定时刷新会把字刷掉）。
    const curEd = editId ? document.querySelector('#queue .task.editing .ed-body') : null;
    const keepEd = (curEd && editId) ? {
      id: editId,
      body: curEd.value || '',
      key: (document.querySelector('#queue .task.editing .ed-target') || {}).value || '',
    } : null;
    const q = await api().Queue() || [];
    const box = $('queue');
    box.innerHTML = '';
    // 显示**倒序**。
    // 派发顺序没变（还是按 prio 从队首往下扫），只是卡片反过来画
    // 所以**最下面那条先派**；「排队第 N」仍按派发顺序算（N 小的先派）。
    $('queue-hint').textContent = q.length
      ? tr('共 ') + q.length + tr(' 条 · 新的在上 · 最下面那条先派')
      : tr('队列为空');
    if (!q.length) { editId = 0; box.innerHTML = `<div class="empty">${tr('队列为空。在上方填写任务内容，按 Ctrl+Enter 加入队列。')}</div>`; return; }
    q.slice().reverse().forEach((t, i) => {
      const c = taskCard({ ...t, prio: q.length - i }, 'queued');
      box.appendChild(c);
    });
    if (keepEd) {
      const ta = box.querySelector('.task.editing .ed-body');
      const sel = box.querySelector('.task.editing .ed-target');
      if (ta && keepEd.id === editId) ta.value = keepEd.body;
      if (sel && keepEd.key && [...sel.options].some((o) => o.value === keepEd.key)) sel.value = keepEd.key;
    }
    if (dbgTab === 'debug') dbgCheckQueue(box, q);
  } catch (e) { feed(tr('读队列失败：') + e.message); }
}

// 界面自检（--tab debug）：确认按钮真的渲染出来了，并真点一次「上移」「删除」
// 验证「前端点击 → 调 Go → 刷新」这条链通不通。
let dbgRan = false;
async function dbgCheckQueue(box, q) {
  const acts = box.querySelector('.acts');
  const card = box.querySelector('.task');
  await api().Debug('排队 ' + q.length + ' 条 · 卡片宽=' + (card ? Math.round(card.getBoundingClientRect().width) : '-')
    + ' · 按钮行数=' + (acts ? acts.querySelectorAll('button').length : 0)
    + ' · 窗口宽=' + window.innerWidth);
  if (dbgRan || q.length < 2 || !acts) return;
  dbgRan = true;
  // 这里的顺序都是**后端给的派发顺序**（旧→新），跟屏幕上倒着画无关。
  const order = () => q.map((t) => t.id).join(',');
  const shown = () => [...box.querySelectorAll('.task .tid')].map((x) => x.textContent).join(',');
  await api().Debug('点之前顺序: ' + order() + ' · 屏幕上(新的在上): ' + shown());
  const last = q[q.length - 1];
  const up = box.querySelector(`button[data-act="up"][data-id="${last.id}"]`);
  if (up) up.click();
  await new Promise((r) => setTimeout(r, 1200));
  const after = await api().Queue() || [];
  await api().Debug('点击「上移」后: ' + after.map((t) => t.id).join(','));
  const lastAfter = after[after.length - 1];
  const cancel = lastAfter
    ? document.querySelector(`#queue button[data-act="cancel"][data-id="${lastAfter.id}"]`)
    : null;
  if (cancel) cancel.click();
  await new Promise((r) => setTimeout(r, 1200));
  const left = await api().Queue() || [];
  await api().Debug('点击「删除」后剩余 ' + left.length + ' 条: ' + left.map((t) => t.id).join(','));
  await dbgCheckEdit();
}

/* 就地编辑自检（T-020）：点「编辑」→ 卡片自己变输入框，上方新建任务里正在写的字不能被冲掉；
   改完按「改好了」→ 正文与发送窗口都存上、位置不变；按「不改了」→ 一个字都不变。
   测试任务由命令行先造好（造之前已暂停派发，不会真发出去）。 */
let dbgEditRan = false;
async function dbgCheckEdit() {
  if (dbgEditRan) return;
  dbgEditRan = true;
  try {
    const box = $('queue');
    const btns = [...box.querySelectorAll('button[data-act="copy"]')];
    if (btns.length < 2) { await api().Debug('[编辑自检] 排队不足 2 条，跳过'); return; }
    const id = Number(btns[btns.length - 1].dataset.id);
    const before = ((await api().Queue()) || []).find((t) => t.id === id);
    if (!before) return;
    // 「原位置」按**派发顺序**算（屏幕上新的在上，DOM 里最后一个按钮＝派发顺序第1位
    // 拿按钮个数当原位置会算错一位）。
    const beforeIdx = ((await api().Queue()) || []).findIndex((t) => t.id === id);
    $('body').value = '自检占位：新建任务里正在写的字'; // 模拟用户正在上方写新任务
    btns[btns.length - 1].click();
    await new Promise((r) => setTimeout(r, 1200));
    const ta = box.querySelector('.task.editing .ed-body');
    const sel = box.querySelector('.task.editing .ed-target');
    await api().Debug(`[编辑自检] 点编辑后 新建任务里=[${$('body').value}]（要保住那句占位） 输入框=${ta ? '在' : '没出来'} 原内容带进来=${ta ? (ta.value === before.body ? '是' : '否') : '-'} 窗口选中=${sel ? sel.value : '-'}（要 ${before.targetKey}） 可选窗口=${sel ? sel.options.length : 0}个`);
    if (ta) ta.value = ta.value + '·改过了';
    let newKey = before.targetKey;
    if (sel && sel.options.length > 1) {
      const other = [...sel.options].find((o) => o.value !== before.targetKey);
      if (other) { sel.value = other.value; newKey = other.value; }
    }
    box.querySelector('.task.editing .ed-save').click();
    await new Promise((r) => setTimeout(r, 1500));
    const after = await api().Queue() || [];
    const at = after.findIndex((t) => t.id === id);
    const t2 = after[at];
    await api().Debug(`[编辑自检] 按改好了后 派发顺序第${at + 1}位（原第${beforeIdx + 1}位，要同一位） 正文=[${t2 ? t2.body : '-'}]（要带·改过了） 窗口=[${t2 ? t2.targetKey : '-'}]（要 ${newKey}） 状态=[${t2 ? t2.state : '-'}]（要 queued） 新建任务里=[${$('body').value}]（还是那句占位）`);
    // 再进一次编辑走「不改了」：库里内容必须一个字不变。
    const again = document.querySelector(`#queue button[data-act="copy"][data-id="${id}"]`);
    if (again) { again.click(); await new Promise((r) => setTimeout(r, 1200)); }
    const ta2 = document.querySelector('#queue .task.editing .ed-body');
    if (ta2) ta2.value = '这段乱写的要被丢掉';
    const drop = document.querySelector('#queue .task.editing .ed-cancel');
    if (drop) { drop.click(); await new Promise((r) => setTimeout(r, 1500)); }
    const back = (await api().Queue() || []).find((t) => t.id === id);
    await api().Debug(`[编辑自检] 按不改了后 正文=[${back ? back.body : '-'}]（不能带「乱写的」） 状态=[${back ? back.state : '-'}]（要 queued）`);
    await api().Debug('[编辑自检] 结束（测试任务随后用命令行清）');
  } catch (e) { await api().Debug('[编辑自检] 跑不动: ' + e.message); }
}

async function refreshDispatched() {
  try {
    const d = await api().Dispatched() || [];
    // 一份数据分两堆：**还在跑的**留「已派发待回复」，**判完工的**进「已完成」。
    // 收了回执、还是只凭窗口空闲判完（done_unacked），**都算完成**，只是标签写明区别。
    // 三个区都**新的在上**：后端给的是旧→新，这里倒过来画。
    const pend = d.filter((t) => t.state === 'dispatched').reverse();
    const fin = d.filter((t) => t.state !== 'dispatched').reverse();
    const box = $('dispatched');
    box.innerHTML = '';
    if (!pend.length) { box.innerHTML = `<div class="empty">${tr('没有待回复的任务。')}</div>`; }
    else { for (const t of pend) box.appendChild(taskCard(t, 'dispatched')); }
    const done = $('donelist');
    done.innerHTML = '';
    if (!fin.length) { done.innerHTML = `<div class="empty">${tr('本轮还没有完成的任务。')}</div>`; }
    else {
      $('done-hint').textContent = tr('共 ') + fin.length + tr(' 条已完成（没收到回执的也在这里，标签会写明）。点内容可展开看全文。');
      for (const t of fin) done.appendChild(taskCard(t, 'done'));
    }
  } catch (e) { feed(tr('读取执行中任务失败：') + e.message); }
}

let pickedFiles = []; // 待发附件（选/拖进来的，加入队列后清空）

async function addTask() {
  const key = $('target').value;
  const body = $('body').value.trim();
  if (!key) { feed(tr('请先去「窗口」页勾选要派工的窗口。')); return; }
  if (!body && !pickedFiles.length) { feed(tr('任务内容尚未填写。')); return; }
  try {
    const id = pickedFiles.length
      ? await api().AddTaskFiles(key, body || tr('请看这个文件（已随本条一起发出）'), pickedFiles)
      : await api().AddTask(key, body);
    $('body').value = '';
    setPickedFiles([]);
    feed(tr('已加入队列 ') + `T-${String(id).padStart(3, '0')}` + tr('，目标窗口空闲时会自动派发。'));
    await Promise.all([refreshQueue(), refreshOverview()]);
  } catch (e) { feed(tr('加入队列失败：') + e.message); }
}

// 待发附件区：图片显缩略图，其他文件显类型块；每个右上角一个叉可单独拿掉。
function setPickedFiles(paths) {
  pickedFiles = (paths || []).filter(Boolean);
  const box = $('pending-files'), n = $('file-note');
  if (!box) return;
  if (!pickedFiles.length) {
    box.classList.add('hidden'); box.innerHTML = '';
    if (n) { n.classList.add('hidden'); n.textContent = ''; }
    return;
  }
  box.classList.remove('hidden');
  box.innerHTML = '';
  pickedFiles.forEach((p, i) => {
    const name = p.split(/[\\/]/).pop();
    const cell = document.createElement('div');
    cell.className = 'pend';
    cell.innerHTML = (isImagePath(p)
      ? `<img data-i="${i}" src="" alt="${esc(name)}" title="${tr('点一下放大')}">`
      : `<span class="fi" data-i="${i}" title="${tr('点一下放大')}">${esc(extOf(p) || tr('文件'))}</span>`) +
      `<span class="nm" title="${esc(p)}">${esc(name)}</span>` +
      `<button class="x" data-i="${i}" title="${tr('不要这个附件')}">×</button>`;
    box.appendChild(cell);
    if (isImagePath(p)) {
      const im = cell.querySelector('img');
      api().Thumb(p).then((u) => { if (u) im.src = u; else im.outerHTML = `<span class="fi" data-i="${i}">${tr('图')}</span>`; })
        .catch(() => { im.outerHTML = `<span class="fi" data-i="${i}">${tr('图')}</span>`; });
    }
  });
  if (n) {
    const total = pickedFiles.length;
    n.classList.remove('hidden');
    n.textContent = tr('准备随任务发出 ') + total + tr(' 个文件（点缩略图放大，点右上角 × 拿掉）');
  }
}

function isImagePath(p) { return /\.(png|jpe?g|gif|bmp|webp)$/i.test(p || ''); }
function extOf(p) { const m = /\.([A-Za-z0-9]{1,5})$/.exec(p || ''); return m ? m[1].toLowerCase() : ''; }

// 待发附件区：点 × 拿掉，点图/类型块放大。
if ($('pending-files')) $('pending-files').addEventListener('click', async (ev) => {
  const x = ev.target.closest('button.x');
  if (x) {
    const i = Number(x.dataset.i);
    const gone = pickedFiles[i];
    setPickedFiles(pickedFiles.filter((_, k) => k !== i));
    feed(tr('已拿掉附件：') + (gone || '').split(/[\\/]/).pop());
    return;
  }
  const t = ev.target.closest('img, .fi');
  if (t) await openLightbox(Number(t.dataset.i));
});

// 截图「完成」后先进待发区（和传文件一样），让用户先写要说的话再自己点「加入队列」。
window.addShotToPending = (path) => {
  if (!path) return;
  setPickedFiles(pickedFiles.concat([path]));
  openTab('dispatch');
  const b = $('body');
  if (b) b.focus();
};

async function pickFiles() {
  try {
    const ps = await api().PickFiles();
    if (!ps || !ps.length) return; // 用户取消
    const warn = await api().CheckFiles(ps);
    if (warn && warn.indexOf('发不出去') === 0) { feed(warn); return; }
    setPickedFiles(pickedFiles.concat(ps));
    if (warn) feed(warn);
  } catch (e) { feed(tr('选文件失败：') + e.message); }
}

/* ---------- 放大看图（点缩略图） ---------- */

async function openLightbox(i) {
  const p = pickedFiles[i];
  if (!p) return;
  await showLightbox(p, await api().FileData(p));
}

async function showLightbox(path, dataUrl) {
  const box = $('lightbox');
  if (!box) return;
  if (!dataUrl) { feed(tr('这个类型不能在这里放大看（图片才可以）。')); return; }
  $('lightbox-img').src = dataUrl;
  $('lightbox-name').textContent = path.split(/[\\/]/).pop();
  box.classList.remove('hidden');
}

function closeLightbox() {
  const box = $('lightbox');
  if (box) { box.classList.add('hidden'); $('lightbox-img').src = ''; }
}

// 拖文件进来：Go 那边开了 Wails 拖放，拖到输入区就会推这个事件。
function bindDropZone() {
  const z = document.querySelector('.composer');
  if (!z || !window.runtime || !window.runtime.EventsOn) return;
  window.runtime.EventsOn('drop-files', async (paths) => {
    const ps = paths || [];
    if (!ps.length) return;
    const warn = await api().CheckFiles(ps);
    if (warn && warn.indexOf('发不出去') === 0) { feed(tr('拖进来的文件：') + warn); return; }
    setPickedFiles(pickedFiles.concat(ps));
    feed(tr('拖进来 ') + ps.length + tr(' 个文件，已列入待发附件。'));
    if (warn) feed(warn);
  });
  // 高亮：Wails 自己给可放元素加 .wails-drop-target-active。
  const mo = new MutationObserver(() => z.classList.toggle('dropping', z.classList.contains('wails-drop-target-active')));
  mo.observe(z, { attributes: true, attributeFilter: ['class'] });
}

/* 队列上的按钮（上移/下移/删除/编辑）统一在这里接。 */
$('queue').addEventListener('click', async (ev) => {
  const b = ev.target.closest('button[data-act]');
  if (!b) return;
  const id = Number(b.dataset.id);
  const q = await api().Queue() || [];
  const idx = q.findIndex((t) => t.id === id);
  try {
    if (b.dataset.act === 'copy') {
      // 编辑：在这条卡片里就地改（不再往上方新建任务里搬，免得把正在写的字冲掉）。
      // 这条转成「正在编辑」：引擎先跳过它，改完按「改好了」回到原位置继续排队。
      if (!windowsCache.length) { try { windowsCache = await api().Windows() || []; } catch (_) { /* 取不到就用空的 */ } }
      await api().StartEdit(id);
      editId = id;
      await refreshQueue();
      const ta = $('queue').querySelector('.task.editing .ed-body');
      if (ta) { ta.focus(); ta.setSelectionRange(ta.value.length, ta.value.length); }
      feed(`T-${String(id).padStart(3, '0')} ` + tr('已转入就地编辑，这条先不派发；改完按「改好了」重新排队。'));
      await refreshOverview();
      return;
    }
    if (b.dataset.act === 'cancel') {
      await api().CancelTask(id);
      feed(`T-${String(id).padStart(3, '0')} ` + tr('已删除。'));
    } else if (b.dataset.act === 'up') {
      if (idx <= 0) return;
      // 「上移」＝插到「紧前面那条」之前（传上上条会跳两格）。q 是后端给的派发顺序，与屏幕画法无关。
      await api().MoveTask(id, q[idx - 1].id);
      feed(`T-${String(id).padStart(3, '0')} ` + tr('已前移。'));
    } else if (b.dataset.act === 'down') {
      if (idx < 0 || idx >= q.length - 1) return;
      // 「下移」＝插到「后面第二条」之前；后面只剩一条时就是挪到队尾（传 0 会被当队首）。
      await api().MoveTask(id, idx + 2 < q.length ? q[idx + 2].id : -1);
      feed(`T-${String(id).padStart(3, '0')} ` + tr('已后移。'));
    }
    await Promise.all([refreshQueue(), refreshOverview()]);
  } catch (e) { feed(tr('操作失败：') + e.message); }
});

/* ---------- 窗口页 ---------- */

async function refreshWindows() {
  try {
    windowsCache = await api().Windows() || [];
  } catch (e) { feed(tr('读窗口失败：') + e.message); return; }
  renderTargetSelect();
  const box = $('windows');
  box.innerHTML = '';
  if (!windowsCache.length) { box.innerHTML = `<div class="empty">${tr('未扫描到窗口，请确认 OpenClaw 网关正在运行。')}</div>`; return; }
  for (const w of windowsCache) {
    const div = document.createElement('div');
    div.className = 'win' + (w.included ? '' : ' off');
    const modelText = (w.model || tr('模型未知')) + (w.modelKind ? '（' + w.modelKind + '）' : '');
    const meta = [esc(modelText), w.pending ? tr('待处理输入 ') + w.pending + tr(' 条') : '', ago(w.updatedAt)]
      .filter(Boolean).join('　·　');
    div.innerHTML = `
      <input type="checkbox" data-key="${esc(w.key)}" ${w.included ? 'checked' : ''}>
      <div><div class="name">${esc(w.label)}</div><div class="key">${esc(w.key)}</div>
        <div class="meta ${w.modelKind === '本地离线' ? 'off' : ''}">${meta}</div></div>
      <span class="tag ${w.idle ? 'ok' : 'warn'}">${w.idle ? tr('空闲') : tr('执行中')}</span>`;
    box.appendChild(div);
  }
}

$('windows').addEventListener('change', async (ev) => {
  const cb = ev.target.closest('input[data-key]');
  if (!cb) return;
  try {
    await api().SetIncluded(cb.dataset.key, cb.checked);
    feed(tr('已') + (cb.checked ? tr('纳入') : tr('移出')) + tr('派工范围。'));
    await refreshWindows();
  } catch (e) { feed(tr('改勾选失败：') + e.message); }
});

/* ---------- 日志页 ---------- */

async function refreshLog() {
  try {
    const lines = await api().RecentLog(300) || [];
    logLines = lines.slice().reverse();
    renderLog();
    const el = $('log');
    el.scrollTop = el.scrollHeight;
  } catch (e) { $('log').textContent = tr('读日志失败：') + e.message; }
}

// 日志内容跟着界面语言翻（C 阶段）：日志是本程序自己写的运行记录，不是 OpenClaw 那边发来的文字
// 所以英文界面下翻；行里的窗口名、文件路径、报错原文不在词典里，保持原文。
function renderLog() {
  const el = $('log');
  if (!el) return;
  const on = window.__i18n && window.__i18n.getLang && window.__i18n.getLang() === 'en';
  el.textContent = (on ? logLines.map((l) => window.__i18n.trLog(l)) : logLines).join('\n') || tr('暂无记录。');
  el.scrollTop = el.scrollHeight;
}

/* ---------- 设置页 ---------- */

async function loadSettings() {
  try {
    const s = await api().GetSettings();
    settingsCache = s;
    $('s-interval').value = s.intervalSec;
    $('s-parallel').value = s.maxParallel;
    $('s-recheck').value = s.recheckSec;
    $('s-retry').value = s.retryDelaySec;
    $('s-target').value = s.notifyTarget || '';
    $('s-tail').checked = !!s.appendTail;
    $('s-gate').checked = !!s.gateService;
    $('s-gate-gpu').checked = !!s.gateGpu;
    $('s-gpu-pct').value = s.gpuPercent || 30;
    $('s-auto').checked = !!s.autoInclude;
    $('s-tray').checked = !!s.trayOnClose;
    $('s-shutdown').checked = !!s.autoShutdown;
    $('s-shours').value = s.shutdownHours || '';
    $('s-mute').value = s.muteMode || '';
    $('s-lang').value = s.lang || 'zh';
    const sel = $('s-channel');
    sel.innerHTML = `<option value="">${tr('不发送通知（仅记录日志）')}</option>`;
    const known = ['feishu', 'openclaw-weixin', 'telegram', 'discord', 'slack', 'whatsapp', 'signal', 'matrix'];
    const list = Array.from(new Set([...(s.channels || []), ...known]));
    for (const c of list) {
      const o = document.createElement('option');
      o.value = c;
      o.textContent = c + (s.channels && s.channels.includes(c) ? '' : tr('（本机未配置）'));
      sel.appendChild(o);
    }
    sel.value = s.notifyChannel || '';
    await loadTargets(true);
    await loadAutoStart();
  } catch (e) { $('settings-hint').textContent = tr('读设置失败：') + e.message; }
}

/* 收件人下拉。
   名单从本机 OpenClaw 现读（按当前选的通道筛）；选一项就把它的 target 填进手填框，
   保存的仍是 target——后端 resolveNotifyRef 本来就按 target 查地址，不用改发送逻辑。 */
async function loadTargets(keepValue) {
  const pick = $('s-target-pick');
  const note = $('target-note');
  const ch = $('s-channel').value;
  if (!ch) {
    pick.innerHTML = `<option value="">${tr('（选了通道才能查名单）')}</option>`;
    note.textContent = tr('选了通知通道后，这里会列出本机 OpenClaw 已知的会话供你选。');
    return;
  }
  pick.innerHTML = `<option value="">${tr('正在查本机已知会话…')}</option>`;
  try {
    const list = await api().ListConversations(ch) || [];
    pick.innerHTML = '';
    const o0 = document.createElement('option');
    o0.value = '';
    o0.textContent = list.length ? tr('（手填，不用名单里的）') : tr('（没查到，请手填）');
    pick.appendChild(o0);
    for (const c of list) {
      const o = document.createElement('option');
      o.value = c.target;
      o.textContent = `${c.label || c.target}　（${c.kind === 'group' ? tr('群') : tr('个人')}）`;
      pick.appendChild(o);
    }
    const cur = keepValue ? ($('s-target').value.trim()) : '';
    if (cur && list.some((c) => c.target === cur)) pick.value = cur;
    note.textContent = list.length
      ? tr('已列出 ') + list.length + tr(' 个「') + ch + tr('」会话；选一个自动填到下面，也可以继续手填。')
      : tr('没查到「') + ch + tr('」的会话。可能这个通道还没收过消息；可在下面手填地址，或先在那个通道里发一条消息再点「刷新名单」。');
  } catch (e) {
    pick.innerHTML = `<option value="">${tr('查不到名单（可继续手填）')}</option>`;
    note.textContent = tr('查名单失败：') + e.message + tr('。不影响手填地址。');
  }
}

async function loadAutoStart() {
  try {
    const a = await api().GetAutoStart();
    $('s-startup').checked = !!a.enabled;
    if (!a.enabled) {
      $('startup-note').textContent = tr('当前未开启。勾选后下次开机自动运行（只写当前用户的启动项，不动系统服务，随时可取消）。');
      return;
    }
    // 路径是从开机启动登记处现读的，不是写死的；程序被搬过/改名过会提醒重新勾一次。
    $('startup-note').textContent = a.stale
      ? tr('已开启，但登记的是旧位置：') + a.regPath + tr('（当前运行的是 ') + a.exePath + tr('）——请取消勾选再勾一次，把登记更新过来。')
      : tr('已开启，开机时自动启动的就是当前这份：') + a.regPath;
  } catch (e) {
    $('startup-note').textContent = tr('读开机启动状态失败：') + e.message;
  }
}

$('s-channel').addEventListener('change', () => { loadTargets(true); });

// 「刷新通道」：软件一直开着时用户新配了通道（微信/飞书），点一下重读配置。
$('btn-channel-refresh').addEventListener('click', async () => {
  const btn = $('btn-channel-refresh');
  btn.disabled = true;
  $('settings-hint').textContent = tr('正在重读本机的通道配置…');
  try {
    await loadSettings();
    await refreshProbe();
    $('settings-hint').textContent = tr('通道已重读；选通道会自动带出该通道的收件地址。');
  } catch (e) { $('settings-hint').textContent = tr('重读通道失败：') + e.message; }
  btn.disabled = false;
});

$('btn-target-refresh').addEventListener('click', async () => {
  $('btn-target-refresh').disabled = true;
  await loadTargets(true);
  $('btn-target-refresh').disabled = false;
});

// 下拉选一个会话 → 把它的地址填进手填框（保存的仍是地址，发送逻辑不用改）。
$('s-target-pick').addEventListener('change', (ev) => {
  if (ev.target.value) $('s-target').value = ev.target.value;
});

$('s-startup').addEventListener('change', async (ev) => {
  try {
    await api().SetAutoStart(ev.target.checked);
    $('settings-hint').textContent = ev.target.checked ? tr('开机自动启动已开启') : tr('开机自动启动已取消');
    await loadAutoStart();
  } catch (e) {
    ev.target.checked = !ev.target.checked;
    $('settings-hint').textContent = tr('设置开机自动启动失败：') + e.message;
  }
});

$('btn-save').addEventListener('click', async () => {
  try {
    await api().SaveSettings({
      intervalSec: Number($('s-interval').value),
      maxParallel: Number($('s-parallel').value),
      recheckSec: Number($('s-recheck').value),
      retryDelaySec: Number($('s-retry').value),
      notifyChannel: $('s-channel').value,
      notifyTarget: $('s-target').value.trim(),
      appendTail: $('s-tail').checked,
      gateService: $('s-gate').checked,
      gateGpu: $('s-gate-gpu').checked,
      gpuPercent: Number($('s-gpu-pct').value) || 30,
      autoInclude: $('s-auto').checked,
      trayOnClose: $('s-tray').checked,
      autoShutdown: $('s-shutdown').checked,
      shutdownHours: $('s-shours').value.trim(),
      muteMode: $('s-mute').value,
      lang: $('s-lang').value,
    });
    // 界面语言：保存后立刻生效（不用重启）。
    if (window.__i18n) window.__i18n.setLang($('s-lang').value);
    $('settings-hint').textContent = tr('已保存 ') + fmtTime(Date.now());
    feed(tr('设置已保存。'));
    await refreshOverview();
    await refreshQueue();
    await refreshDispatched();
    await refreshWindows();
    await loadSettings();
    await refreshLog(); // 语言一换，日志页的文字要跟着换（日志页开着时立刻见效）
  } catch (e) { $('settings-hint').textContent = tr('保存失败：') + e.message; }
});

// exportRange 在 app.go；这里只决定下拉里有什么。
$('btn-export').addEventListener('click', async () => {
  $('settings-hint').textContent = tr('正在导出…');
  try {
    const r = await api().ExportCSV($('export-scope').value);
    $('settings-hint').textContent = tr(r || '未导出（已取消）');
  } catch (e) { $('settings-hint').textContent = tr('导出失败：') + e.message; }
});

/* 这台机器没探测到聊天通道 → 给两个选择。 */
async function refreshProbe() {
  try {
    const p = await api().Probe();
    $('probe').classList.toggle('hidden', !p.needChoice);
  } catch (e) { /* 探测失败不拦其它刷新 */ }
}
$('probe-none').addEventListener('click', async () => {
  try { await api().ChooseNoNotify(); $('probe').classList.add('hidden'); feed(tr('本机不发送通知，结果仅记录在日志中。')); await refreshOverview(); } catch (e) { feed(tr('设置失败：') + e.message); }
});
$('probe-retry').addEventListener('click', async () => {
  try { await api().RetryProbe(); await refreshProbe(); await refreshOverview(); } catch (e) { feed(tr('重新检测失败：') + e.message); }
});

$('btn-notify-test').addEventListener('click', async () => {
  $('settings-hint').textContent = tr('正在发…');
  try {
    await api().NotifyTest(tr('【派工台】这是一条测试通知，收到即表示通道正常。'));
    $('settings-hint').textContent = tr('已发送，请到收件处查看。');
  } catch (e) { $('settings-hint').textContent = tr('发送失败：') + e.message; }
});

/* ---------- 底栏 ---------- */

$('btn-toggle').addEventListener('click', async () => {
  try {
    if (await api().IsRunning()) await api().StopLoop(); else await api().StartLoop();
    await refreshOverview();
  } catch (e) { feed(tr('切换失败：') + e.message); }
});

$('btn-tick').addEventListener('click', async () => {
  $('btn-tick').disabled = true;
  feed(tr('正在检查一轮…'));
  try {
    const msg = await api().TickNow();
    await Promise.all([refreshQueue(), refreshDispatched(), refreshOverview(), refreshWindows()]);
    feed(tr(msg || '已检查一轮。'));
  } catch (e) { feed(tr('本次检查出错：') + e.message); }
  $('btn-tick').disabled = false;
});

/* ---------- 页签 ---------- */

function openTab(name) {
  const t = document.querySelector(`.tab[data-tab="${name}"]`);
  if (t) t.click();
}

document.querySelectorAll('.tab').forEach((t) => {
  t.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach((x) => x.classList.remove('active'));
    document.querySelectorAll('.page').forEach((x) => x.classList.remove('active'));
    t.classList.add('active');
    $('page-' + t.dataset.tab).classList.add('active');
    if (t.dataset.tab === 'log') refreshLog();
    if (t.dataset.tab === 'settings') loadSettings();
    if (t.dataset.tab === 'windows') refreshWindows();
  });
});

/* ---------- 快捷键 ---------- */

$('btn-add').addEventListener('click', addTask);
if ($('btn-file')) $('btn-file').addEventListener('click', pickFiles);
if ($('lightbox-close')) $('lightbox-close').addEventListener('click', closeLightbox);
if ($('lightbox')) $('lightbox').addEventListener('click', (ev) => { if (ev.target === $('lightbox')) closeLightbox(); });
window.addEventListener('keydown', (ev) => {
  const lb = $('lightbox');
  if (ev.key === 'Escape' && lb && !lb.classList.contains('hidden')) { ev.preventDefault(); closeLightbox(); }
});
$('body').addEventListener('keydown', (ev) => {
  if (ev.key === 'Enter' && (ev.ctrlKey || ev.metaKey)) { ev.preventDefault(); addTask(); }
});

/* ---------- 后端推的事件 ---------- */

let eventsBound = false;
function bindEvents() {
  if (eventsBound || !window.runtime || !window.runtime.EventsOn) return;
  eventsBound = true;
  window.runtime.EventsOn('feed', (msg) => {
    feed(msg);
    if ($('page-log').classList.contains('active')) refreshLog();
  });
  window.runtime.EventsOn('refresh', () => {
    Promise.all([refreshOverview(), refreshQueue(), refreshDispatched(), refreshWindows()]);
  });
  // 托盘菜单直接开某一页（Go 那边叫 OpenTab 发这个事件）。
  window.runtime.EventsOn('tab', (name) => { openTab(name); });
  bindDropZone();
}

/* ---------- 启动 ---------- */

// 顶栏自检（--tab debug）：把品牌/标签页/徐章的位置报进日志。
// 直接量元素比裁图猜可靠（界面版无控制台，Debug 已落进事件日志，用 catherder.exe log 读）。
async function dbgReportTop() {
  try {
    const vw = window.innerWidth;
    const rect = (el) => {
      if (!el) return '-';
      const r = el.getBoundingClientRect();
      return `[${Math.round(r.left)},${Math.round(r.top)} ${Math.round(r.width)}x${Math.round(r.height)}]`;
    };
    const tabs = [...document.querySelectorAll('.tab')].map((t) => t.textContent + rect(t)).join(' ');
    const chips = [...document.querySelectorAll('.statusbar .chip')].map((c) => c.id + rect(c)).join(' ');
    const logo = document.querySelector('.logo img');
    const name = document.querySelector('.brand-name');
    await api().Debug(`顶栏自检 视口宽=${vw} 品牌=${rect(document.querySelector('.brand'))} 标签页=${tabs} 徐章=${chips}`);
    await api().Debug(`图标=${rect(logo)} 文字=${rect(name)}`);
    // 界面语言自检：切英文→标签页要变英文→切回中文→要还原。
    // 比 textContent（标签页本来就没 title，拿 title 比会拿到空值——探测脚本自己也会错）。
    if (window.__i18n) {
      const tab0 = document.querySelector('.tab[data-tab="dispatch"]');
      const zh = tab0 ? tab0.textContent : '';
      window.__i18n.setLang('en');
      const en = tab0 ? tab0.textContent : '';
      window.__i18n.setLang('zh');
      const back = tab0 ? tab0.textContent : '';
      const keep = tab0 ? (tab0.getAttribute('data-zh-text') || '(无)') : '(无)';
      const lab = document.querySelector('#page-settings .lab');
      const labEn = (() => { window.__i18n.setLang('en'); const v = lab ? lab.textContent : ''; window.__i18n.setLang('zh'); return v; })();
      await api().Debug(`[语言自检] 中文=[${zh}] → 英文=[${en}]（要 Dispatch）→ 切回=[${back}]（要还原成 ${zh}） 按钮宽=${tab0 ? Math.round(tab0.getBoundingClientRect().width) : 0} 存档=${keep} 设置页标签英文=[${labEn}]`);
      // 整条链：设置页选 English → 点保存 → 界面真变英文 → 重读设置里 lang 真是 en → 改回中文。
      // **先 loadSettings()**：设置页还没加载过时表单是空的，直接点保存会把空值写进库（踩过：几个开关被清成 0）。
      const sel = $('s-lang');
      if (sel) {
        await loadSettings();
        sel.value = 'en';
        $('btn-save').click();
        await new Promise((r) => setTimeout(r, 2000)); // 保存是异步的，等它跑完再看
        const afterEn = document.querySelector('.tab[data-tab="dispatch"]').textContent;
        const ss = await api().GetSettings();
        await api().Debug(`[语言自检] 点保存后 标签页=[${afterEn}]（要 Dispatch）设置里 lang=[${ss.lang}]（要 en）`);
        sel.value = 'zh';
        $('btn-save').click();
        await new Promise((r) => setTimeout(r, 2000));
        await api().Debug(`[语言自检] 改回中文 标签页=[${document.querySelector('.tab[data-tab="dispatch"]').textContent}]（要 派工）`);
      }
      // 日志页（C 阶段）：英文界面下日志行要变英文，窗口名/路径/报错原文要保住。
      try {
        window.__i18n.setLang('en');
        await refreshLog();
        // 取最后两条「不是 [前端] 调试行」的日志（调试行按设计原样不翻，拿它当样本看不出效果）。
        const pick = () => ($('log').textContent || '').split('\n').filter((x) => x && x.indexOf('[前端]') < 0).slice(-2).join(' ⏏ ');
        const enLog = pick();
        window.__i18n.setLang('zh');
        await refreshLog();
        const zhLog = pick();
        await api().Debug(`[日志自检] 英文=[${enLog}] 中文=[${zhLog}]`);
      } catch (e) { await api().Debug('[日志自检] 跑不动: ' + e.message); }
    }
  } catch (e) { /* 自检失败不影响正常使用 */ }
}

let booted = false;
async function boot() {
  bindEvents();
  try {
    // 界面语言：先按存好的设置把静态页面换过来，再渲染各页（不然启动时是中文）。
    try {
      const ss = await api().GetSettings();
      if (window.__i18n) window.__i18n.setLang(ss.lang || 'zh');
    } catch (_) { /* 读不到就用默认中文 */ }
    const tab = await api().StartTab();
    dbgTab = tab;
    window.dbgTab = tab; // 截图模块（shot.js）也要知道开了调试页没有
    if (tab && tab !== 'debug') {
      const t = document.querySelector(`.tab[data-tab="${tab}"]`);
      if (t) t.click();
    }
    await refreshWindows();
    await Promise.all([refreshOverview(), refreshQueue(), refreshDispatched()]);
    const stamp = await api().BuildStamp();
    let ver = '';
    try { ver = await api().BuildVersion(); } catch (_) { /* 旧版没这个方法就不显版本 */ }
    if ($('build-stamp')) {
      $('build-stamp').textContent = (ver ? tr('版本 ') + ver + ' · ' : '') + (stamp ? tr('本版编译于 ') + stamp : '');
    }
    booted = true;
    if (dbgTab === 'debug') {
      dbgReportTop();
      // 截图整条链靠鼠标拖框，脚本代不了；让页面自己模拟事件跑一遗。
      // 必须在这里调：shot.js 绑定比 boot() 早，那时 dbgTab 还没拿到值。
      if (window.__shot) setTimeout(() => window.__shot.selftest(), 1200);
    }
  } catch (e) {
    feed(tr('程序后台尚未就绪：') + e.message);
    setTimeout(boot, 1500);
  }
}
boot();

// 后端偶尔慢启动：没连上就每秒再试一次，连上后自动刷新。
setInterval(() => { if (!booted) boot(); }, 1500);
