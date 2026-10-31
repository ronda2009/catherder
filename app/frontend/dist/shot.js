/* 截图 + 标注。
 *
 * 流程：点「截图」→ Go 抓整屏并把自己全屏 → 这一帧铺满屏幕（等于把屏幕冻住）
 * → 拖鼠标框选 → 画笔/方框/画圈/箭头/文字 → 「完成」把选区+标注合成一张图
 * → 存盘 → 作为带图任务加入队列 → 派发时文字和图一起发给目标窗口。
 *
 * 坐标规矩（搞错就会糊或错位）：
 * 抓回来的图是物理像素；界面里一切按 CSS 像素算。
 * scale = 图宽 / 视口宽。画的时候用 CSS 像素，出图时按 scale 放大回物理像素
 * 所以标注不会糊。
 */
(function () {
  const $ = (id) => document.getElementById(id);

  let img = null; // 整屏那一帧
  let scale = 1; // CSS 像素 → 图片物理像素
  let sel = null; // 选区 {x,y,w,h}，CSS 像素、相对视口
  let marks = []; // 标注（CSS 像素、相对视口）
  let redo = []; // 被撤销掉的标注，用于「重做」
  let tool = 'pen';
  let color = '#ff3b30';
  let size = 4; // 线宽（细/中/粗）
  let drawing = false;
  let cur = null; // 正在拖的那一笔
  let textEl = null; // 文字输入框（落点挂在 textEl.at 上）
  let pickEl = null; // 取色器的小卡片
  let picked = ''; // 取色器刚取到的色值（Ctrl+C 复制用）
  let mosaicImg = null; // 整屏马赛克版，涂抹时按笔迹裁开填
  let busy = false;

  const canvas = () => $('shot-canvas');
  const shotHidden = () => $('shot').classList.contains('hidden');

  // 放大镜：直接画在主画布上（少一个元素、少一套定位，就不会出不来）
  const LOUPE_W = 220, LOUPE_H = 168, LOUPE_BAR = 44, LOUPE_CELL = 6;
  let loupeAt = null; // 放大镜要跟的鼠标点（CSS 像素）；null = 不显
  let loupeInfo = { shown: false, x: 0, y: 0, hex: '' };

  /* ---------- 开关 ---------- */

  async function startShot() {
    if (busy) return;
    busy = true;
    try {
      const s = await api().CaptureScreen(); // 只抓屏，还不全屏
      // 黑底先盖住界面再全屏：撑大、标签页重排全在黑底背后。
      sel = null; marks = []; redo = []; mosaicImg = null;
      $('shot').classList.remove('hidden');
      img = new Image();
      img.onload = async () => {
        fitCanvas();
        setStage('sel');
        redraw();
        await api().EnterShot(); // 全屏后窗口 resize 会自己再 fitCanvas
      };
      img.onerror = () => {
        busy = false;
        api().ExitShot();
        $('shot').classList.add('hidden');
        feed('底图解不开，截图取消。');
      };
      img.src = 'data:image/png;base64,' + s.b64;
    } catch (e) {
      busy = false;
      $('shot').classList.add('hidden');
      try { await api().ExitShot(); } catch (_) { /* 没全屏成功就不用退 */ }
      feed('截图失败：' + e.message);
    }
  }

  async function endShot() {
    hideLoupe();
    dropText();
    dropPick();
    picked = '';
    // 工具也要复位成花圈：漏了这步就会：用户取色后按 Esc 取消、再截图出不来框。
    // 点一下花圈按钮＝复用现成的切换（连高亮与光标一起改），不新写一套。
    const pen = document.querySelector('[data-tool="pen"]');
    if (pen) pen.click();
    // 颜色也复位成默认红：同样复用现成的红块点击，不新写一套。
    const red = document.querySelector('.shot-color[data-color="#ff3b30"]');
    if (red) red.click();
    sel = null; marks = []; redo = []; drawing = false; cur = null; mosaicImg = null;
    busy = false;
    try { await api().ExitShot(); } catch (_) { /* 退不了也不拦 */ }
    $('shot').classList.add('hidden');
  }

  /* ---------- 画布尺寸 ---------- */

  function fitCanvas() {
    const c = canvas();
    const vw = window.innerWidth, vh = window.innerHeight;
    const dpr = window.devicePixelRatio || 1;
    c.width = Math.round(vw * dpr);
    c.height = Math.round(vh * dpr);
    c.style.width = vw + 'px';
    c.style.height = vh + 'px';
    scale = img ? img.naturalWidth / vw : dpr; // 以实际抓到的图宽为准，比猜 dpr 可靠
  }

  /* ---------- 重画 ---------- */

  function redraw() {
    const c = canvas();
    if (!c || !img) return;
    const dpr = window.devicePixelRatio || 1;
    const ctx = c.getContext('2d');
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0); // 之后全按 CSS 像素画
    const vw = window.innerWidth, vh = window.innerHeight;
    ctx.clearRect(0, 0, vw, vh);
    ctx.drawImage(img, 0, 0, vw, vh);

    if (sel && sel.w > 2 && sel.h > 2) {
      // 选区外盖一层灰，选区内重画原图（放大后更清楚）
      ctx.fillStyle = 'rgba(0,0,0,0.45)';
      ctx.fillRect(0, 0, vw, sel.y);
      ctx.fillRect(0, sel.y, sel.x, sel.h);
      ctx.fillRect(sel.x + sel.w, sel.y, vw - sel.x - sel.w, sel.h);
      ctx.fillRect(0, sel.y + sel.h, vw, vh - sel.y - sel.h);
      ctx.save();
      ctx.beginPath();
      ctx.rect(sel.x, sel.y, sel.w, sel.h);
      ctx.clip();
      ctx.drawImage(img, sel.x * scale, sel.y * scale, sel.w * scale, sel.h * scale,
        sel.x, sel.y, sel.w, sel.h);
      marks.forEach((m) => drawMark(ctx, m));
      if (cur) drawMark(ctx, cur);
      ctx.restore();
      ctx.strokeStyle = '#4c8dff';
      ctx.lineWidth = 2;
      ctx.strokeRect(sel.x, sel.y, sel.w, sel.h);
      ctx.fillStyle = '#e6e9ef';
      ctx.font = '12px "Microsoft YaHei", sans-serif';
      ctx.fillText(tr('选区 ') + Math.round(sel.w) + '×' + Math.round(sel.h) + tr(' · 花圈/椭圆/方形/箭头/文字/马赛克/取色，Ctrl+Z 撤销，Ctrl+Y 重做，回车完成，Esc 取消'),
        sel.x + 4, Math.max(14, sel.y - 6));
    } else {
      marks.forEach((m) => drawMark(ctx, m));
      if (cur) drawMark(ctx, cur);
    }
    // 放大镜画不画全看 loupeAt；每帧先当没画，真画了 drawLoupe 会自己报上。
    // （不复位就会拿着上一帧的旧值报「还在」，探测就信了假。）
    loupeInfo.shown = false;
    if (loupeAt) drawLoupe(ctx, loupeAt);
  }

  /* ---------- 一笔 ---------- */

  function drawMark(ctx, m) {
    ctx.strokeStyle = m.color;
    ctx.fillStyle = m.color;
    ctx.lineWidth = m.size || size;
    ctx.lineJoin = 'round';
    ctx.lineCap = 'round';
    const p = m.pts;
    if (m.tool === 'mosaic') {
      // 涂抹：笔路上每一点放一个圆，圆合起来就是裁剪区，把整屏马赛克填进去 → 涂到哪哪是马赛克。

      const mo = mosaicImg || buildMosaic();
      if (!mo) return;
      const r = (m.size || 4) * 3; // 圆圈大小跟着「细/中/粗」变
      ctx.save();
      ctx.beginPath();
      for (const q of p) { ctx.moveTo(q.x + r, q.y); ctx.arc(q.x, q.y, r, 0, Math.PI * 2); }
      ctx.clip();
      ctx.drawImage(mo, 0, 0, mo.width, mo.height, m.ox || 0, m.oy || 0, window.innerWidth, window.innerHeight);
      ctx.restore();
      return;
    }
    if (m.tool === 'pen') {
      if (p.length < 2) { dot(ctx, p[0]); return; }
      ctx.beginPath();
      ctx.moveTo(p[0].x, p[0].y);
      for (let i = 1; i < p.length; i++) ctx.lineTo(p[i].x, p[i].y);
      ctx.stroke();
      return;
    }
    const a = p[0], b = p[p.length - 1];
    if (m.tool === 'sel') { // 框选的橡皮框：按住拖动就要看见，不是松手才出
      ctx.strokeStyle = '#4c8dff';
      ctx.lineWidth = 2;
      ctx.strokeRect(Math.min(a.x, b.x), Math.min(a.y, b.y), Math.abs(b.x - a.x), Math.abs(b.y - a.y));
      return;
    }
    if (m.tool === 'rect') {
      ctx.strokeRect(Math.min(a.x, b.x), Math.min(a.y, b.y), Math.abs(b.x - a.x), Math.abs(b.y - a.y));
      return;
    }
    if (m.tool === 'ellipse') {
      ctx.beginPath();
      ctx.ellipse((a.x + b.x) / 2, (a.y + b.y) / 2,
        Math.abs(b.x - a.x) / 2, Math.abs(b.y - a.y) / 2, 0, 0, Math.PI * 2);
      ctx.stroke();
      return;
    }
    if (m.tool === 'arrow') {
      const ang = Math.atan2(b.y - a.y, b.x - a.x);
      const hl = Math.max(10, (m.size || size) * 4); // 箭头大小跟着「细/中/粗」变
      ctx.beginPath();
      ctx.moveTo(a.x, a.y);
      ctx.lineTo(b.x, b.y);
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(b.x, b.y);
      ctx.lineTo(b.x - hl * Math.cos(ang - Math.PI / 6), b.y - hl * Math.sin(ang - Math.PI / 6));
      ctx.moveTo(b.x, b.y);
      ctx.lineTo(b.x - hl * Math.cos(ang + Math.PI / 6), b.y - hl * Math.sin(ang + Math.PI / 6));
      ctx.stroke();
      return;
    }
    if (m.tool === 'text') {
      ctx.font = `${m.size || 18}px "Microsoft YaHei", "Segoe UI", sans-serif`;
      ctx.fillText(m.text, a.x, a.y); // 就填一个色，不加描边不加风格
    }
  }

  // 整屏马赛克只算一次，涂抹时按笔迹裁开填，不每笔重算。
  function buildMosaic() {
    if (!img) return null;
    const blk = 12;
    const tmp = document.createElement('canvas');
    tmp.width = Math.max(1, Math.round(img.naturalWidth / blk));
    tmp.height = Math.max(1, Math.round(img.naturalHeight / blk));
    tmp.getContext('2d').drawImage(img, 0, 0, img.naturalWidth, img.naturalHeight, 0, 0, tmp.width, tmp.height);
    const mo = document.createElement('canvas');
    mo.width = img.naturalWidth; mo.height = img.naturalHeight;
    const mc = mo.getContext('2d');
    mc.imageSmoothingEnabled = false;
    mc.drawImage(tmp, 0, 0, tmp.width, tmp.height, 0, 0, mo.width, mo.height);
    mosaicImg = mo;
    return mo;
  }

  function dot(ctx, p) {
    ctx.beginPath();
    ctx.arc(p.x, p.y, ctx.lineWidth / 2, 0, Math.PI * 2);
    ctx.fill();
  }

  /* ---------- 鼠标 ---------- */

  function pos(ev) {
    const r = canvas().getBoundingClientRect();
    return { x: ev.clientX - r.left, y: ev.clientY - r.top };
  }

  function onDown(ev) {
    if (ev.button !== 0) return;
    const p = pos(ev);
    // 正在打字时点到别处：先把这段字落定（保留），再在这一点开新的输入框。
    if (textEl) { commitText(); openText(p); return; }
    if (!sel) { // 还没选区：不管选的什么工具，按下都先框选。
      drawing = true; cur = { tool: 'sel', pts: [p] }; // （「卡在取色器上」：取色后取消再截图
      return; // 工具还停在取色 → 按下只取色不框选 → 出不来框）
    }
    if (tool === 'pick') { pickColor(p); return; } // 取色不受选区限制（微信那样）
    if (!inSel(p)) return; // 选区外不管：先选区再标注
    if (tool === 'text') { ev.preventDefault(); hideLoupe(); openText(p); redraw(); return; }
    drawing = true;
    loupeAt = null; // 拖标注时放大镜让开，不挡要画的地方
    cur = { tool, color, size, pts: [p] };
  }

  function onMove(ev) {
    if (shotHidden() || textEl) return;
    const p = pos(ev);
    if (drawing && cur) {
      const last = cur.pts[cur.pts.length - 1];
      // 没挪够 1 像素就不记点也不重画：点少一半，重画就快。
      if (Math.abs(p.x - last.x) < 1 && Math.abs(p.y - last.y) < 1) return;
      loupeAt = cur.tool === 'sel' ? p : null;
      cur.pts.push(p);
      redraw();
      return;
    }
    // 放大镜只在「还没框选区」时跟鼠标走（框选要精细到坐标）；选区定完就没
    // 除非正取色（取色本来就要看清那个像素）。
    loupeAt = (!sel || tool === 'pick') ? p : null;
    redraw();
  }

  function onUp() {
    if (!drawing || !cur) return;
    drawing = false;
    if (cur.tool === 'sel') {
      const a = cur.pts[0], b = cur.pts[cur.pts.length - 1];
      const w = Math.abs(b.x - a.x), h = Math.abs(b.y - a.y);
      cur = null;
      if (w < 6 || h < 6) { redraw(); return; } // 点一下不算选区
      sel = { x: Math.min(a.x, b.x), y: Math.min(a.y, b.y), w, h };
      setStage('mark');
      redraw();
      return;
    }
    marks.push(cur);
    redo = [];
    cur = null;
    redraw();
  }

  function inSel(p) {
    return sel && p.x >= sel.x && p.x <= sel.x + sel.w && p.y >= sel.y && p.y <= sel.y + sel.h;
  }

  /* ---------- 文字 ---------- */

  function openText(p) {
    textEl = document.createElement('textarea');
    textEl.className = 'shot-text';
    textEl.rows = 1;
    textEl.at = p; // 落点直接挂在输入框上，不再单独记一个变量
    textEl.style.left = p.x + 'px';
    textEl.style.top = (p.y - 18) + 'px';
    textEl.style.color = color;
    textEl.style.caretColor = color;
    $('shot').appendChild(textEl);
    textEl.focus();
    textEl.addEventListener('keydown', (ev) => {
      ev.stopPropagation();
      if (ev.key === 'Enter' && !ev.shiftKey) { ev.preventDefault(); commitText(); }
      if (ev.key === 'Escape') { dropText(); }
    });
    textEl.addEventListener('blur', commitText);
  }

  function commitText() {
    if (!textEl) return;
    const t = textEl.value.trim();
    const at = textEl.at;
    const el = textEl;
    textEl = null;
    el.remove();
    if (t) { marks.push({ tool: 'text', color, size: 18, pts: [at], text: t }); redo = []; }
    redraw();
  }

  function dropText() {
    if (textEl) { const el = textEl; textEl = null; el.remove(); }
  }

  /* ---------- 取色器 ---------- */

  // 从底图取一个点的颜色（底图是物理像素，坐标要乘 scale）。
  // 1x1 探针画布只建一次：放大镜每帧都要取色，每帧新建画布是白扔性能。
  const probe = document.createElement('canvas');
  probe.width = probe.height = 1;
  const probeCtx = probe.getContext('2d');

  function sampleColor(p) {
    try {
      probeCtx.drawImage(img, Math.round(p.x * scale), Math.round(p.y * scale), 1, 1, 0, 0, 1, 1);
      const d = probeCtx.getImageData(0, 0, 1, 1).data;
      return d[3] === 0 ? null : rgbHex(d[0], d[1], d[2]);
    } catch (e) { return null; }
  }

  // 读已画出来的像素；坐标已是该 ctx 的实际像素。
  function readPixel(ctx, x, y) {
    try {
      const d = ctx.getImageData(Math.round(x), Math.round(y), 1, 1).data;
      return d[3] === 0 ? null : rgbHex(d[0], d[1], d[2]);
    } catch (e) { return null; }
  }

  // 主画布上的 CSS 坐标 → 实际像素（dpr）。
  function canvasColorAt(x, y) {
    return readPixel(canvas().getContext('2d'), Math.round(x * (window.devicePixelRatio || 1)), Math.round(y * (window.devicePixelRatio || 1)));
  }

  function rgbHex(r, g, b) {
    const h = (n) => n.toString(16).padStart(2, '0');
    return '#' + h(r) + h(g) + h(b);
  }

  function pickColor(p) {
    const hex = sampleColor(p);
    if (!hex) { feed('这个点取不到颜色。'); return; }
    picked = hex;
    dropPick();
    pickEl = document.createElement('div');
    pickEl.className = 'shot-pick';
    pickEl.innerHTML = `<div>${tr('坐标 ')}${Math.round(p.x)}, ${Math.round(p.y)}</div>` +
      `<div>${tr('色值 ')}<i style="background:${hex}"></i><b>${hex}</b></div>` +
      `<div class="k">${tr('按 Ctrl+C 复制色值')}</div>`;
    pickEl.style.left = Math.max(4, Math.min(p.x + 14, window.innerWidth - 200)) + 'px';
    pickEl.style.top = Math.max(4, Math.min(p.y + 14, window.innerHeight - 96)) + 'px';
    $('shot').appendChild(pickEl);
    // 取到的颜色只亮在色块排里给用户点用，**不偷偷改画笔色**（用户报「默认红，点两回都是白」
    // 先取色就把画笔改成了桌面那个近白色）。
    const sw = $('shot-color-custom');
    if (sw) {
      sw.style.background = hex;
      sw.dataset.color = hex;
      sw.classList.remove('hidden');
    }
  }

  function dropPick() {
    if (pickEl) { const el = pickEl; pickEl = null; el.remove(); }
  }

  /* ---------- 放大镜 ---------- */

  function hideLoupe() {
    loupeAt = null;
    loupeInfo.shown = false;
  }

  // 放大镜直接画在主画布上（redraw 末尾调）：没有单独元素、没有单独定位，就不会出不来。
  function drawLoupe(ctx, p) {
    if (!img) return;
    const cell = LOUPE_CELL;
    const srcW = Math.floor(LOUPE_W / cell), srcH = Math.floor((LOUPE_H - LOUPE_BAR) / cell);
    const drawW = srcW * cell, drawH = srcH * cell;
    const cx = Math.round(p.x * scale), cy = Math.round(p.y * scale); // 图上的像素坐标
    const sx = Math.max(0, Math.min(cx - (srcW >> 1), img.naturalWidth - srcW));
    const sy = Math.max(0, Math.min(cy - (srcH >> 1), img.naturalHeight - srcH));
    let left = p.x + 20, top = p.y + 20;
    if (left + LOUPE_W > window.innerWidth) left = p.x - LOUPE_W - 20;
    if (top + LOUPE_H > window.innerHeight) top = p.y - LOUPE_H - 20;
    left = Math.max(0, Math.round(left)); top = Math.max(0, Math.round(top));

    ctx.save();
    ctx.fillStyle = 'rgba(8,10,14,.92)';
    ctx.fillRect(left, top, LOUPE_W, LOUPE_H);
    ctx.imageSmoothingEnabled = false;
    ctx.drawImage(img, sx, sy, srcW, srcH, left, top, drawW, drawH);
    ctx.imageSmoothingEnabled = true;

    const midX = left + (cx - sx + 0.5) * cell, midY = top + (cy - sy + 0.5) * cell;
    ctx.strokeStyle = 'rgba(255,255,255,.75)';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(left, midY); ctx.lineTo(left + drawW, midY);
    ctx.moveTo(midX, top); ctx.lineTo(midX, top + drawH);
    ctx.stroke();
    ctx.strokeStyle = '#ff3b30';
    ctx.strokeRect(midX - cell / 2, midY - cell / 2, cell, cell);

    const hex = sampleColor(p) || '';
    const barY = top + LOUPE_H - LOUPE_BAR;
    ctx.fillStyle = 'rgba(8,10,14,.92)';
    ctx.fillRect(left, barY, LOUPE_W, LOUPE_BAR);
    if (hex) { ctx.fillStyle = hex; ctx.fillRect(left + 7, barY + 7, 30, 30); }
    ctx.fillStyle = '#e6e9ef';
    ctx.font = '12px "Microsoft YaHei", sans-serif';
    ctx.textBaseline = 'middle';
    ctx.fillText(tr('坐标 ') + cx + ', ' + cy, left + 45, barY + 13);
    ctx.fillText(hex ? tr('色值 ') + hex : tr('色值 —'), left + 45, barY + 31);
    ctx.restore();

    loupeInfo = { shown: true, x: cx, y: cy, hex };
  }

  function undo() {
    if (!marks.length) return;
    redo.push(marks.pop());
    redraw();
  }

  function redoLast() {
    if (!redo.length) return;
    marks.push(redo.pop());
    redraw();
  }

  /* ---------- 出图 ---------- */

  function exportPNG() {
    const out = document.createElement('canvas');
    out.width = Math.max(1, Math.round(sel.w * scale));
    out.height = Math.max(1, Math.round(sel.h * scale));
    const c2 = out.getContext('2d');
    c2.drawImage(img, Math.round(sel.x * scale), Math.round(sel.y * scale), out.width, out.height, 0, 0, out.width, out.height);
    c2.scale(scale, scale); // 之后按 CSS 像素画标注
    marks.forEach((m) => drawMarkAt(c2, m, -sel.x, -sel.y));
    return out.toDataURL('image/png');
  }

  // 与 drawMark 同一套画法，只是整体平移（把视口坐标挪成选区内坐标）
  function drawMarkAt(ctx, m, dx, dy) {
    drawMark(ctx, {
      tool: m.tool, color: m.color, size: m.size, text: m.text,
      ox: dx, oy: dy, // 马赛克整屏铺图也要跟着平移
      pts: m.pts.map((q) => ({ x: q.x + dx, y: q.y + dy })),
    });
  }

  /* ---------- 完成 / 取消 ---------- */

  async function finish() {
    if (!sel) { feed('先用鼠标框出要截的区域。'); return; }
    dropText();
    let dataURL;
    try { dataURL = exportPNG(); } catch (e) { feed('合成图片失败：' + e.message); return; }
    await endShot();
    try {
      const path = await api().SaveShot(dataURL);
      // 不直接排队：跟传文件一样先进待发区，让用户先写要说的话再点「加入队列」。
      if (window.addShotToPending) window.addShotToPending(path);
      feed('截图已放进待发区（和传文件一样）：先写要说的话，再点「加入队列」。');
    } catch (e) { feed('存图失败：' + e.message); }
  }

  // 另存为：弹系统「另存为」对话框让用户选地方。
  async function saveOnly() {
    if (!sel) { feed('先用鼠标框出要截的区域。'); return; }
    dropText();
    let dataURL;
    try { dataURL = exportPNG(); } catch (e) { feed('合成图片失败：' + e.message); return; }
    await endShot();
    try {
      const path = await api().SaveShotAs(dataURL);
      feed(path ? `图已存到：${path}` : '没存（你在对话框里取消了）');
    } catch (e) { feed('存图失败：' + e.message); }
  }

  /* ---------- 界面状态 ---------- */

  // 工具栏跟着选区走：默认放选区下方居中，下方放不下就放上方，再放不下压在选区底里。
  function placeBar() {
    const bar = document.querySelector('.shot-bar');
    if (!bar) return;
    if (!sel) {
      bar.classList.add('bar-bottom');
      bar.style.left = bar.style.top = bar.style.bottom = '';
      return;
    }
    bar.classList.remove('bar-bottom');
    const bw = bar.offsetWidth, bh = bar.offsetHeight;
    let top = sel.y + sel.h + 8;
    if (top + bh > window.innerHeight - 4) top = sel.y - bh - 8;
    if (top < 4) top = Math.max(4, Math.min(sel.y + sel.h - bh - 8, window.innerHeight - bh - 4));
    let left = sel.x + (sel.w - bw) / 2;
    left = Math.max(4, Math.min(left, window.innerWidth - bw - 4));
    bar.style.left = Math.round(left) + 'px';
    bar.style.top = Math.round(top) + 'px';
    bar.style.bottom = 'auto';
  }

  // 光标跟工具走：文字=竖线，取色/花圈/椭圆/方形=细十字。
  // 马赛克的圆圈现给（不写死在 CSS 里）：直径跟「细/中/粗」变，跟涂出来的圆一样大。
  const CURSORS = { pen: 'cur-draw', rect: 'cur-draw', ellipse: 'cur-draw', arrow: 'cur-draw', mosaic: 'cur-mosaic', text: 'cur-text', pick: 'cur-pick' };
  function mosaicCursor(sz) {
    const d = Math.min(sz * 6, 32); // Windows 自定义光标最大就 32 像素，粗挡到底
    const h = d / 2, r = h - 2;
    return `url('data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="${d}" height="${d}">` +
      `<circle cx="${h}" cy="${h}" r="${r}" fill="rgba(255,255,255,0.15)" stroke="black" stroke-width="2"/>` +
      `<circle cx="${h}" cy="${h}" r="${r - 1}" fill="none" stroke="white" stroke-width="1.5"/></svg>') ${h} ${h}, cell`;
  }
  function setCursor() {
    const c = canvas();
    if (!c) return;
    Object.keys(CURSORS).forEach((k) => c.classList.remove(CURSORS[k]));
    c.classList.add(CURSORS[tool] || 'cur-draw');
    c.style.cursor = tool === 'mosaic' ? mosaicCursor(size) : '';
  }

  function setStage(stage) {
    const tip = $('shot-tip'), done = $('shot-done');
    if (stage === 'sel') {
      tip.textContent = tr('第 1 步：拖动鼠标框出要截的区域（放大镜显精确坐标，Esc 取消）');
      done.disabled = true;
    } else {
      tip.textContent = tr('第 2 步：标注（Ctrl+Z 撤销、Ctrl+Y 重做）；点「完成」送进待发区，不直接排队');
      done.disabled = false;
    }
    placeBar();
  }

  function bind() {
    const c = canvas();
    setCursor();
    c.addEventListener('mousedown', onDown);
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);

    $('shot-row-tools').addEventListener('click', (ev) => {
      const b = ev.target.closest('button[data-tool]');
      if (!b) return;
      tool = b.dataset.tool;
      document.querySelectorAll('.shot-tool').forEach((x) => x.classList.toggle('active', x === b));
      setCursor();
      dropText();
      dropPick(); // 「取色卡框卡着」：切工具时要把取色卡片清掉
    });
    // 按下不抢输入框焦点（preventDefault）：打字中也能换颜色，文字不落定、不消失。
    $('shot-colors').addEventListener('mousedown', (ev) => { if (textEl) ev.preventDefault(); });
    $('shot-colors').addEventListener('click', (ev) => {
      const s = ev.target.closest('.shot-color');
      if (!s) return;
      color = s.dataset.color;
      document.querySelectorAll('.shot-color').forEach((x) => x.classList.toggle('active', x === s));
      if (textEl) { textEl.style.color = color; textEl.style.caretColor = color; } // 正在打的字直接跟着变色
    });
    $('shot-undo').addEventListener('click', undo);
    $('shot-redo').addEventListener('click', redoLast);
    $('shot-clear').addEventListener('click', () => { marks = []; redo = []; dropPick(); redraw(); });
    $('shot-sizes').addEventListener('click', (ev) => {
      const b = ev.target.closest('.shot-size');
      if (!b) return;
      size = Number(b.dataset.size) || 4;
      document.querySelectorAll('.shot-size').forEach((x) => x.classList.toggle('active', x === b));
      setCursor(); // 换了粗细，马赛克圆圈光标也要跟着变大小
    });
    $('shot-done').addEventListener('click', finish);
    $('shot-save').addEventListener('click', saveOnly);
    $('shot-cancel').addEventListener('click', endShot);

    window.addEventListener('keydown', async (ev) => {
      if ($('shot').classList.contains('hidden')) return;
      if (textEl) return; // 打字时不抢键
      if (ev.key === 'Escape') { ev.preventDefault(); endShot(); }
      else if (ev.key === 'Enter') { ev.preventDefault(); finish(); }
      else if (ev.key === 'z' && (ev.ctrlKey || ev.metaKey)) { ev.preventDefault(); undo(); }
      else if (ev.key === 'y' && (ev.ctrlKey || ev.metaKey)) { ev.preventDefault(); redoLast(); }
      else if (ev.key === 'c' && (ev.ctrlKey || ev.metaKey) && picked) {
        ev.preventDefault();
        try { await api().CopyText(picked); feed('色值已复制：' + picked); } catch (e) { feed('复制失败：' + e.message); }
      }
    });
    window.addEventListener('resize', () => { if (!$('shot').classList.contains('hidden')) { fitCanvas(); redraw(); placeBar(); } });

    const btn = $('btn-shot');
    if (btn) btn.addEventListener('click', startShot);
    window.__shot = { start: startShot, selftest: selftest };

    // 界面自检（--tab debug）：自己跑一遗截图流程，把每步结果报进日志。
    if (window.dbgTab === 'debug') setTimeout(selftest, 1200);
  }

  // 自检做到「存盘」为止，不加入队列：加入队列就会真发消息给窗口，不该自检顺带发。
  async function selftest() {
    const say = (m) => { try { api().Debug('[截图自检] ' + m); } catch (_) { /* 后端没就绪就算了 */ } };
    try {
      const s = await api().CaptureScreen();
      say(`抓屏 ${s.width}x${s.height} base64 长 ${s.b64.length}`);
      img = new Image();
      img.onload = async () => {
        try {
          const before = { w: window.innerWidth, h: window.innerHeight };
          $('shot').classList.remove('hidden');
          fitCanvas();
          setStage('sel');
          redraw();
          await api().EnterShot();
          await new Promise((r) => setTimeout(r, 250));
          fitCanvas();
          say(`全屏后视口 ${window.innerWidth}x${window.innerHeight}（全屏前 ${before.w}x${before.h}）画布 ${canvas().width}x${canvas().height} scale=${scale.toFixed(3)}`);
          // 模拟拖框：左上到右下取中间一块
          const vw = window.innerWidth, vh = window.innerHeight;
          const mk = (type, x, y) => new MouseEvent(type, { button: 0, clientX: x, clientY: y, bubbles: true });
          sel = null; marks = []; redo = [];
          canvas().dispatchEvent(mk('mousedown', vw * 0.2, vh * 0.2));
          window.dispatchEvent(mk('mousemove', vw * 0.6, vh * 0.55));
          // 必须在松手前探：①橡皮框要已经画出来②放大镜要在。
          const rbBlue = [-2, -1, 0, 1, 2].some((d) => canvasColorAt(vw * 0.2 + d, vh * 0.38) === '#4c8dff');
          say(`框选拖动中橡皮框${rbBlue ? '已画出' : '没画出'}；放大镜${loupeInfo.shown ? '在' : '没显示'} 中心 ${loupeInfo.x},${loupeInfo.y} 色 ${loupeInfo.hex || '—'}`);
          window.dispatchEvent(mk('mouseup', vw * 0.6, vh * 0.55));
          say(`选区 ${JSON.stringify(sel)}`);
          const bar = document.querySelector('.shot-bar');
          const br = bar ? bar.getBoundingClientRect() : null;
          const rowBtns = bar ? bar.querySelectorAll('.shot-row').length : 0;
          const acts = document.querySelector('.shot-actions');
          const ar = acts ? acts.getBoundingClientRect() : null;
          const row2 = acts ? acts.closest('.shot-row').getBoundingClientRect() : null;
          const rightGap = ar && row2 ? Math.round(row2.right - ar.right) : -1;
          say(`工具栏 ${br ? Math.round(br.left) + ',' + Math.round(br.top) + ' ' + Math.round(br.width) + 'x' + Math.round(br.height) : '没找到'} 选区底=${sel.y + sel.h} 屏高=${vh} → ${br && br.top >= sel.y + sel.h - 2 ? '在选区下方' : (br && br.bottom <= sel.y + 2 ? '在选区上方' : '没贴住选区')}；行数=${rowBtns} 撤销那组靠右=${rightGap >= 0 && rightGap <= 8 ? '是' : '否'}（距行右 ${rightGap}px）另存为在第二行=${acts && acts.contains($('shot-save')) ? '是' : '否'}`);
          window.dispatchEvent(mk('mousemove', vw * 0.55, vh * 0.6));
          say(`选区定完后放大镜${loupeInfo.shown ? '还在（应该没）' : '已隐'}`);
          // 模拟一笔自由线
          tool = 'pen'; color = '#ff3b30'; size = 4;
          canvas().dispatchEvent(mk('mousedown', vw * 0.3, vh * 0.3));
          window.dispatchEvent(mk('mousemove', vw * 0.4, vh * 0.4));
          window.dispatchEvent(mk('mouseup', vw * 0.4, vh * 0.4));
          say(`标注 ${marks.length} 笔，拖标注时放大镜${loupeInfo.shown ? '没让开' : '已让开'}`);
          // 箭头：拖一条线，末端两翼要真有色（没两翼就是普通直线）。工具靠点工具条按钮切，连接线一起验。
          const arrowBtn = document.querySelector('[data-tool="arrow"]');
          if (arrowBtn) arrowBtn.click(); else { tool = 'arrow'; setCursor(); }
          const ax = vw * 0.25, ay = vh * 0.44, bx = vw * 0.5, by = vh * 0.5;
          canvas().dispatchEvent(mk('mousedown', ax, ay));
          window.dispatchEvent(mk('mousemove', vw * 0.4, vh * 0.47));
          window.dispatchEvent(mk('mouseup', bx, by));
          const am = marks[marks.length - 1];
          // 探点用这一笔自己记的端点（mouseup 的坐标不会追加进笔里，拿它算就探偏）。
          const ap = am.pts[0], bp = am.pts[am.pts.length - 1];
          const ah = Math.max(10, (am.size || size) * 4);
          const ag = Math.atan2(bp.y - ap.y, bp.x - ap.x);
          // 两翼终点算法与 drawMark 一致，再沿「终点→尖端」取中点探色，探的就是线上。
          const wingPt = (sgn, f) => {
            const wx = bp.x - ah * Math.cos(ag + sgn * Math.PI / 6), wy = bp.y - ah * Math.sin(ag + sgn * Math.PI / 6);
            return { x: bp.x + (wx - bp.x) * f, y: bp.y + (wy - bp.y) * f };
          };
          const wingHit = [0.5, 0.75].every((f) => [-1, 1].every((s) => canvasColorAt(wingPt(s, f).x, wingPt(s, f).y) === color));
          say(`箭头 工具按钮=${arrowBtn ? '在' : '没找到'} 切后工具=${tool}；一笔=${am.tool === 'arrow' ? '是' : '否'} 端点=${Math.round(ap.x)},${Math.round(ap.y)}→${Math.round(bp.x)},${Math.round(bp.y)}；线上有色=${canvasColorAt((ap.x + bp.x) / 2, (ap.y + bp.y) / 2) === color ? '是' : '否'}；末端两翼有色=${wingHit ? '是（箭头画出来了）' : '否（没箭头）'}`);
          // 马赛克：拿小圆圈沿笔迹涂。探点故意选在一个马赛克格子的正中，比对才准。
          tool = 'mosaic'; size = 4;
          const blk = 12;
          const px0 = Math.floor(vw * 0.35 * scale / blk) * blk + blk / 2;
          const py0 = Math.floor(vh * 0.35 * scale / blk) * blk + blk / 2;
          const mx = px0 / scale, my = py0 / scale;
          canvas().dispatchEvent(mk('mousedown', mx, my));
          window.dispatchEvent(mk('mousemove', vw * 0.45, vh * 0.45));
          window.dispatchEvent(mk('mouseup', vw * 0.45, vh * 0.45));
          const got = canvasColorAt(mx, my);
          const exp = mosaicImg ? readPixel(mosaicImg.getContext('2d'), px0, py0) : null;
          const out = canvasColorAt(mx - 40, my - 40);
          const outOrig = sampleColor({ x: mx - 40, y: my - 40 });
          say(`马赛克 涂到处 ${got || '—'} 应=马赛克色 ${exp || '—'} → ${got && got === exp ? '对' : '不对'}；笔迹外 ${out || '—'}${out === outOrig ? '=原图（没涂黑）' : '≠原图 ' + outOrig + '（被涂黑了）'}`);
          const src = drawMark.toString();
          say(`画法检查：文字描边=${/strokeText/.test(src) ? '还在（应删）' : '已删'}；马赛克黑描边=${/'#000'/.test(src) ? '还在（应删）' : '已删'}；${marks.length} 笔（涂抹笔点数=${marks[marks.length - 1].pts.length}）`);
          // 文字：点一下应该出输入框并拿到焦点，光标要是文本状
          tool = 'text'; setCursor();
          say(`文字工具光标=${canvas().className || '无'}（要 cur-text）`);
          canvas().dispatchEvent(mk('mousedown', vw * 0.3, vh * 0.5));
          await new Promise((r) => setTimeout(r, 120));
          const focused = textEl && document.activeElement === textEl;
          if (textEl) textEl.value = '第一段';
          // 再点第二个位置：第一段要保留
          canvas().dispatchEvent(mk('mousedown', vw * 0.3, vh * 0.62));
          await new Promise((r) => setTimeout(r, 120));
          const kept1 = marks.filter((m) => m.tool === 'text').length;
          if (textEl) textEl.value = '第二段';
          commitText();
          const texts = marks.filter((m) => m.tool === 'text').map((m) => m.text);
          say(`打字：输入框${focused ? '拿到焦点' : '没拿到焦点'}；两段都在=${texts.length === 2 && texts[0] === '第一段' && texts[1] === '第二段' ? '是' : '否'} → ${JSON.stringify(texts)}`);
          // 打字中换颜色：不能落定、不能丢字，输入框要跟着变色。
          // 探测点必须在选区内：选区外 onDown 直接 return，输入框根本不开。
          canvas().dispatchEvent(mk('mousedown', vw * 0.3, vh * 0.45));
          await new Promise((r) => setTimeout(r, 120));
          const nBefore = marks.length;
          if (textEl) textEl.value = '变色测试';
          const greenSw = document.querySelector('.shot-color[data-color="#34c759"]');
          greenSw.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }));
          const keptFocus = !!textEl && document.activeElement === textEl && marks.length === nBefore;
          greenSw.click();
          await new Promise((r) => setTimeout(r, 80));
          say(`打字中点绿块：文字${keptFocus ? '还在输入（没落定没丢字）' : '被落定/丢了（不对）'}；输入框色=${textEl ? textEl.style.color : '—'}（要 rgb(52, 199, 89)）`);
          if (textEl) { textEl.value = '变色测试'; commitText(); }
          // 马赛克圆圈光标必须跟「细/中/粗」变大小
          // 探法：不拆 style.cursor 的引号（Chromium 会把 " 转义成 \"，拆不到）
          // 直接看浏览器实际生效的 cursor 里 width= 后面那个数。
          tool = 'mosaic';
          const cw = (s) => { const m = /width=\\?["']?(\d+)/.exec(s || ''); return m ? Number(m[1]) : 0; };
          const applied = (sz) => { size = sz; setCursor(); return cw(getComputedStyle(canvas()).cursor); };
          const dFine = applied(2), dMid = applied(4), dFat = applied(8);
          size = 4; setCursor();
          const raw = getComputedStyle(canvas()).cursor.slice(0, 46);
          say(`马赛克光标直径 细=${dFine} 中=${dMid} 粗=${dFat} → ${dFine < dMid && dMid < dFat ? '随粗细变' : '不随粗细变（不对）'}；类=${canvas().className || '无'}（要 cur-mosaic）；生效值=${raw || '空（没挂上）'}`);
          // 取色器：点一下取颜色，再看切工具后卡片会不会清掉
          tool = 'pick'; setCursor(); color = '#ff3b30';
          canvas().dispatchEvent(mk('mousedown', vw * 0.5, vh * 0.5));
          const gotPick = picked;
          say(`取色 ${gotPick || '没取到'}；取色后画笔色=${color}（要还是 #ff3b30，不能被子取到的色偷改）`);
          document.querySelector('[data-tool="mosaic"]').click();
          say(`取色 ${gotPick || '没取到'}，卡片=${pickEl ? '切工具后还卡着（不对）' : '切工具已清掉'}`);
          // 撤销 / 重做
          tool = 'pen';
          undo(); say(`撤销后 ${marks.length} 笔、待重做 ${redo.length} 笔`);
          redoLast(); say(`重做后 ${marks.length} 笔、待重做 ${redo.length} 笔`);
          const url = exportPNG();
          const path = await api().SaveShot(url);
          say(`存盘 OK → ${path}`);
          await endShot();
          say(`退全屏后视口 ${window.innerWidth}x${window.innerHeight}；待发区接应=${window.addShotToPending ? '在' : '没接上'}`);
          // 坑：取色后按 Esc 取消，再截图出不来框（卡在取色器上）→ 专门验一遍。
          tool = 'pick'; setCursor();
          await endShot(); // 模拟取消：看工具会不会被复位
          const s2 = await api().CaptureScreen();
          await new Promise((r) => { img = new Image(); img.onload = r; img.src = 'data:image/png;base64,' + s2.b64; });
          $('shot').classList.remove('hidden'); fitCanvas(); setStage('sel'); redraw();
          await api().EnterShot();
          await new Promise((r) => setTimeout(r, 250));
          fitCanvas(); redraw();
          canvas().dispatchEvent(mk('mousedown', vw * 0.25, vh * 0.25));
          window.dispatchEvent(mk('mousemove', vw * 0.55, vh * 0.5));
          const blue2 = [-2, -1, 0, 1, 2].some((d) => canvasColorAt(vw * 0.25 + d, vh * 0.37) === '#4c8dff');
          say(`第二次截图：取消后工具=${tool}（要 pen，不能留 pick）颜色=${color}（要 #ff3b30 默认红）按下左键橡皮框${blue2 ? '已画出' : '出不来（还卡着）'}`);
          window.dispatchEvent(mk('mouseup', vw * 0.55, vh * 0.5));
          await endShot();
          say('自检完成');
        } catch (e) {
          say('自检失败：' + e.message);
          await endShot();
        }
      };
      img.onerror = () => { say('底图解不开'); endShot(); };
      img.src = 'data:image/png;base64,' + s.b64;
    } catch (e) {
      say('抓屏失败：' + e.message);
      try { await api().ExitShot(); } catch (_) { /* 没全屏成功就不用退 */ }
    } // 给 app.js 里的入口用
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', bind);
  } else {
    bind();
  }
})();
