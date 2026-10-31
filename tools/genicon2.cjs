// 生成阳光版图标 .ico（多尺寸 PNG 帧）。
// 图案：暖黄浅底圆角方块 + 黄色安全帽（工头标志）+ 红色小旗。
// 为什么改：老板 2026-10-24 反馈旧图标「偏黑暗系」，系统主题是暗色时右下角看不清。
// 新底是亮暖黄，暗色任务栏上对比强；边框用中琥珀色，浅色任务栏上也不糊。
const fs = require('fs');
const zlib = require('zlib');

const SIZES = [16, 20, 24, 32, 40, 48, 64];

function crc32(buf) {
  let c, crc = 0xffffffff;
  for (let n = 0; n < buf.length; n++) {
    c = (buf[n] ^ (crc & 0xff)) & 0xff;
    for (let k = 0; k < 8; k++) c = c & 1 ? (c >>> 1) ^ 0xedb88320 : c >>> 1;
    crc = (crc >>> 8) ^ TABLE[c];
  }
  return (crc ^ 0xffffffff) >>> 0;
}
const TABLE = (() => {
  const t = new Uint32Array(256);
  for (let i = 0; i < 256; i++) {
    let c = i;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    t[i] = c >>> 0;
  }
  return t;
})();

function chunk(type, data) {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length);
  const t = Buffer.from(type, 'ascii');
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(Buffer.concat([t, data])));
  return Buffer.concat([len, t, data, crc]);
}

function png(w, h, rgba) {
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4);
  ihdr[8] = 8; ihdr[9] = 6; // 8-bit RGBA
  const raw = Buffer.alloc((w * 4 + 1) * h);
  for (let y = 0; y < h; y++) {
    raw[y * (w * 4 + 1)] = 0;
    rgba.copy(raw, y * (w * 4 + 1) + 1, y * w * 4, (y + 1) * w * 4);
  }
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', zlib.deflateSync(raw, { level: 9 })),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

// 以 48 为基准设计，S 为实际像素边长
function frame(S) {
  const px = Buffer.alloc(S * S * 4);
  const f = S / 48;
  const set = (x, y, r, g, b, a) => {
    if (x < 0 || y < 0 || x >= S || y >= S || a <= 0) return;
    const i = ((y | 0) * S + (x | 0)) * 4;
    const sa = a / 255;
    const da = px[i + 3] / 255;
    const na = sa + da * (1 - sa);
    if (na <= 0) return;
    px[i] = Math.round((r * sa + px[i] * da * (1 - sa)) / na);
    px[i + 1] = Math.round((g * sa + px[i + 1] * da * (1 - sa)) / na);
    px[i + 2] = Math.round((b * sa + px[i + 2] * da * (1 - sa)) / na);
    px[i + 3] = Math.round(na * 255);
  };
  const inCircle = (x, y, cx, cy, r) => {
    const dx = x + 0.5 - cx, dy = y + 0.5 - cy;
    return dx * dx + dy * dy <= r * r;
  };
  const inTri = (a, b, c) => (x, y) => {
    const d1 = (b[0] - a[0]) * (y - a[1]) - (b[1] - a[1]) * (x - a[0]);
    const d2 = (c[0] - b[0]) * (y - b[1]) - (c[1] - b[1]) * (x - b[0]);
    const d3 = (a[0] - c[0]) * (y - c[1]) - (a[1] - c[1]) * (x - c[0]);
    const neg = (v) => v < 0;
    return (neg(d1) && neg(d2) && neg(d3)) || (!neg(d1) && !neg(d2) && !neg(d3));
  };

  // ① 底：暖黄渐变圆角方块（上亮下稍深）+ 中琥珀描边
  const rad = 9 * f;
  const inRound = (x, y) => {
    const l = rad, r = S - 1 - rad, t = rad, bt = S - 1 - rad;
    if (x >= l && x <= r && y >= t && y <= bt) return true;
    const cx = x < l ? l : x > r ? r : x;
    const cy = y < t ? t : y > bt ? bt : y;
    return (x - cx) ** 2 + (y - cy) ** 2 <= rad * rad;
  };
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (!inRound(x, y)) continue;
    const k = y / (S - 1 || 1);
    // #FFF8E6 → #FFD066
    const r = Math.round(255 + (255 - 255) * k);
    const g = Math.round(248 + (208 - 248) * k);
    const b = Math.round(230 + (102 - 230) * k);
    set(x, y, r, g, b, 255);
  }
  // 描边（暗一点的中琥珀，浅色任务栏上也能看清边界）
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (!inRound(x, y)) continue;
    const edge = !inRound(x - 1, y) || !inRound(x + 1, y) || !inRound(x, y - 1) || !inRound(x, y + 1);
    if (edge) set(x, y, 196, 128, 30, 255);
  }

  // ② 脸（肤色圆）
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (inCircle(x, y, 24 * f, 30 * f, 10.5 * f)) set(x, y, 250, 205, 150, 255);
  }

  // ③ 安全帽：上半球（帽壳）+ 帽檐 + 顶部凸棱
  const hat = (x, y) => inCircle(x, y, 24 * f, 23 * f, 11.5 * f) && y + 0.5 <= 23.5 * f;
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (hat(x, y)) set(x, y, 255, 195, 31, 255);
  }
  // 帽檐（比帽壳宽的一横条）
  for (let y = Math.round(22 * f); y < Math.round(25.5 * f); y++) {
    for (let x = Math.round(10 * f); x < Math.round(38 * f); x++) set(x, y, 255, 160, 10, 255);
  }
  // 顶部凸棱（让"安全帽"一眼认得出）
  for (let y = Math.round(12 * f); y < Math.round(22 * f); y++) {
    for (let x = Math.round(22.2 * f); x < Math.round(25.8 * f); x++) set(x, y, 255, 219, 110, 255);
  }

  // ④ 眼睛两点（深棕，别用纯黑，整体更暖）
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (inCircle(x, y, 20.5 * f, 31 * f, 1.7 * f) || inCircle(x, y, 27.5 * f, 31 * f, 1.7 * f)) {
      set(x, y, 74, 48, 24, 255);
    }
  }

  // ⑤ 工头红旗（右上）：旗杆 + 红三角
  for (let y = Math.round(5 * f); y < Math.round(21 * f); y++) {
    for (let x = Math.round(39 * f); x < Math.round(40.4 * f); x++) set(x, y, 122, 92, 52, 255);
  }
  const flag = inTri([39 * f, 6 * f], [39 * f, 14 * f], [31.5 * f, 10 * f]);
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) if (flag(x, y)) set(x, y, 232, 69, 43, 255);

  return px;
}

const frames = SIZES.map((S) => ({ S, data: png(S, S, frame(S)) }));

const hdr = Buffer.alloc(6);
hdr.writeUInt16LE(0, 0); hdr.writeUInt16LE(1, 2); hdr.writeUInt16LE(frames.length, 4);
const dir = Buffer.alloc(frames.length * 16);
let off = 6 + frames.length * 16;
frames.forEach((fr, i) => {
  const base = i * 16;
  dir[base] = fr.S >= 256 ? 0 : fr.S;
  dir[base + 1] = fr.S >= 256 ? 0 : fr.S;
  dir.writeUInt16LE(1, base + 4); // planes
  dir.writeUInt16LE(32, base + 6); // bpp
  dir.writeUInt32LE(fr.data.length, base + 8);
  dir.writeUInt32LE(off, base + 12);
  off += fr.data.length;
});
const ico = Buffer.concat([hdr, dir, ...frames.map((fr) => fr.data)]);

const outs = process.argv.slice(2);
if (!outs.length) outs.push('tray.ico');
for (const o of outs) {
  fs.writeFileSync(o, ico);
  console.log('已写', o, ico.length, 'B', '尺寸:', SIZES.join('/'));
}
