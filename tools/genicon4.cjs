// 生成「戴安全帽的小猫头」图标（老板 2026-10-24 提的：可爱、又对得上 CatHerder 这个名字）。
// 帧格式同 genicon3：未压缩位图帧，程序自己也能读（PNG 帧会被 LoadIcon 判失败）。
const fs = require('fs');

const SIZES = [16, 20, 24, 32, 40, 48, 64];

function frame(S) {
  const px = Buffer.alloc(S * S * 4);
  const f = S / 48;
  const set = (x, y, r, g, b, a) => {
    if (x < 0 || y < 0 || x >= S || y >= S || a <= 0) return;
    const i = ((y | 0) * S + (x | 0)) * 4;
    const sa = a / 255, da = px[i + 3] / 255;
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

  // 底：暖黄渐变圆角方块 + 中琥珀描边（暗任务栏显眼、浅任务栏有边界）
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
    set(x, y, 255, Math.round(248 + (208 - 248) * k), Math.round(230 + (102 - 230) * k), 255);
  }
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (!inRound(x, y)) continue;
    if (!inRound(x - 1, y) || !inRound(x + 1, y) || !inRound(x, y - 1) || !inRound(x, y + 1)) {
      set(x, y, 196, 128, 30, 255);
    }
  }

  // 猫耳（先画耳，再被帽檐压住根部）
  const earL = inTri([13 * f, 12 * f], [8 * f, 24 * f], [22 * f, 20 * f]);
  const earR = inTri([35 * f, 12 * f], [40 * f, 24 * f], [26 * f, 20 * f]);
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (earL(x, y) || earR(x, y)) set(x, y, 255, 176, 60, 255);
  }
  // 耳内（深一点）
  const earLi = inTri([13.5 * f, 15 * f], [10.5 * f, 22.5 * f], [19.5 * f, 20 * f]);
  const earRi = inTri([34.5 * f, 15 * f], [37.5 * f, 22.5 * f], [28.5 * f, 20 * f]);
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (earLi(x, y) || earRi(x, y)) set(x, y, 232, 120, 60, 255);
  }

  // 猫脸（圆，暖橙白）
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (inCircle(x, y, 24 * f, 31 * f, 11 * f)) set(x, y, 255, 226, 178, 255);
  }

  // 安全帽：帽壳半球 + 帽檐 + 顶部凸棱
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (inCircle(x, y, 24 * f, 23.5 * f, 11.5 * f) && y + 0.5 <= 24 * f) set(x, y, 255, 195, 31, 255);
  }
  for (let y = Math.round(22.5 * f); y < Math.round(26 * f); y++) {
    for (let x = Math.round(10 * f); x < Math.round(38 * f); x++) set(x, y, 255, 160, 10, 255);
  }
  for (let y = Math.round(12.5 * f); y < Math.round(22.5 * f); y++) {
    for (let x = Math.round(22.2 * f); x < Math.round(25.8 * f); x++) set(x, y, 255, 219, 110, 255);
  }

  // 眼睛（大一点更可爱）+ 小鼻子
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    if (inCircle(x, y, 20 * f, 32 * f, 2.1 * f) || inCircle(x, y, 28 * f, 32 * f, 2.1 * f)) {
      set(x, y, 60, 40, 22, 255);
    }
    if (inCircle(x, y, 24 * f, 35.5 * f, 1.3 * f)) set(x, y, 232, 110, 110, 255);
  }
  // 胡须（左右各两根，1 像素宽）
  for (let k = 0; k < 2; k++) {
    const yy = Math.round((34 + k * 2.5) * f);
    for (let x = Math.round(11 * f); x < Math.round(18 * f); x++) set(x, yy, 210, 150, 100, 255);
    for (let x = Math.round(30 * f); x < Math.round(37 * f); x++) set(x, yy, 210, 150, 100, 255);
  }

  // 工头红旗（右上，保留「派工」含义）
  for (let y = Math.round(5 * f); y < Math.round(20 * f); y++) {
    for (let x = Math.round(39 * f); x < Math.round(40.4 * f); x++) set(x, y, 122, 92, 52, 255);
  }
  const flag = inTri([39 * f, 6 * f], [39 * f, 14 * f], [31.5 * f, 10 * f]);
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) if (flag(x, y)) set(x, y, 232, 69, 43, 255);

  return px;
}

function dib(S, rgba) {
  const hdr = Buffer.alloc(40);
  hdr.writeUInt32LE(40, 0);
  hdr.writeInt32LE(S, 4);
  hdr.writeInt32LE(S * 2, 8);
  hdr.writeUInt16LE(1, 12);
  hdr.writeUInt16LE(32, 14);
  const xorLen = S * 4 * S;
  hdr.writeUInt32LE(0, 16);
  hdr.writeUInt32LE(xorLen, 20);
  const maskRow = Math.ceil(S / 32) * 4;
  const maskLen = maskRow * S;
  const xor = Buffer.alloc(xorLen);
  for (let y = 0; y < S; y++) {
    const s0 = y * S * 4, d0 = (S - 1 - y) * S * 4;
    for (let x = 0; x < S; x++) {
      const i = s0 + x * 4, o = d0 + x * 4;
      xor[o] = rgba[i + 2]; xor[o + 1] = rgba[i + 1]; xor[o + 2] = rgba[i]; xor[o + 3] = rgba[i + 3];
    }
  }
  return Buffer.concat([hdr, xor, Buffer.alloc(maskLen)]);
}

const frames = SIZES.map((S) => ({ S, data: dib(S, frame(S)) }));
const hdr = Buffer.alloc(6);
hdr.writeUInt16LE(0, 0); hdr.writeUInt16LE(1, 2); hdr.writeUInt16LE(frames.length, 4);
const dir = Buffer.alloc(frames.length * 16);
let off = 6 + frames.length * 16;
frames.forEach((fr, i) => {
  const b = i * 16;
  dir[b] = fr.S; dir[b + 1] = fr.S;
  dir.writeUInt16LE(1, b + 4); dir.writeUInt16LE(32, b + 6);
  dir.writeUInt32LE(fr.data.length, b + 8); dir.writeUInt32LE(off, b + 12);
  off += fr.data.length;
});
const ico = Buffer.concat([hdr, dir, ...frames.map((f) => f.data)]);
for (const o of process.argv.slice(2)) {
  fs.writeFileSync(o, ico);
  console.log('已写', o, ico.length, 'B');
}
