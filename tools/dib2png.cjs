// 把 .ico 里的 BMP(DIB) 帧转成真 PNG，好让模型/人眼直接看小尺寸长什么样。
// 用法: node dib2png.cjs <ico路径> <输出前缀>
const fs = require('fs');
const zlib = require('zlib');

const icoPath = process.argv[2];
const prefix = process.argv[3] || 'frame';
const b = fs.readFileSync(icoPath);
const n = b.readUInt16LE(4);
const out = [];

function crc32(buf) {
  let c, t = new Uint32Array(256);
  for (let i = 0; i < 256; i++) { c = i; for (let k = 0; k < 8; k++) c = c & 1 ? (c >>> 1) ^ 0xedb88320 : c >>> 1; t[i] = c; }
  let x = 0xffffffff;
  for (let i = 0; i < buf.length; i++) x = t[(x ^ buf[i]) & 0xff] ^ (x >>> 8);
  return (x ^ 0xffffffff) >>> 0;
}
function chunk(type, data) {
  const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
  const t = Buffer.from(type);
  const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(Buffer.concat([t, data])));
  return Buffer.concat([len, t, data, crc]);
}
function pngFromRGBA(w, h, rgba) {
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4);
  ihdr[8] = 8; ihdr[9] = 6; // 8bit RGBA
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

for (let i = 0; i < n; i++) {
  const o = 6 + 16 * i;
  const w = b[o] || 256, h = b[o + 1] || 256;
  const bytes = b.readUInt32LE(o + 8), off = b.readUInt32LE(o + 12);
  const hdr = b.readUInt32LE(off);
  if (hdr !== 40) { out.push({ w, h, kind: 'png?' }); continue; }
  const hh = b.readInt32LE(off + 8); // 含 AND mask，通常是 2 倍高
  const imgH = Math.abs(hh) / 2 | 0 || h;
  const px = Buffer.alloc(w * imgH * 4);
  // BGRA，自底向上
  for (let y = 0; y < imgH; y++) {
    const srcRow = off + 40 + (imgH - 1 - y) * w * 4;
    for (let x = 0; x < w; x++) {
      const s = srcRow + x * 4, d = (y * w + x) * 4;
      px[d] = b[s + 2]; px[d + 1] = b[s + 1]; px[d + 2] = b[s]; px[d + 3] = b[s + 3];
    }
  }
  const f = `${prefix}-${w}.png`;
  fs.writeFileSync(f, pngFromRGBA(w, imgH, px));
  out.push({ w, h: imgH, kind: 'bmp', file: f });
}
console.log(JSON.stringify(out));
