// 生成图标 .ico —— 用「老式位图帧」而不是 PNG 帧。
// 为什么：老板 2026-10-24 反馈任务栏里程序图标「像出错了那种」。
// Wails 取窗口/任务栏图标走的是 Win32 LoadIcon(资源里的 RT_ICON)，它只认未压缩 BMP/DIB 帧，
// PNG 压缩帧会被判载入失败 → 图标变通用/破损。外壳（托盘、快捷方式）能读 PNG，但程序自己读不了。
// 所以这里全部输出 BITMAPINFOHEADER + 32bpp BGRA + 1bpp AND 掩码，两头都认。
const fs = require('fs');

const SIZES = [16, 20, 24, 32, 40, 48, 64];

// —— 画法与 genicon2 相同（暖黄底 + 黄色安全帽工头 + 红旗）——
const src = fs.readFileSync(__dirname + '\\genicon2.cjs', 'utf8');
const body = src.slice(src.indexOf('function frame(S)'), src.indexOf('const frames ='));
const frame = new Function('S', body + '\nreturn frame(S);');

function dib(S, rgba) {
  const hdr = Buffer.alloc(40);
  hdr.writeUInt32LE(40, 0);
  hdr.writeInt32LE(S, 4);
  hdr.writeInt32LE(S * 2, 8); // 高 = 彩色 + 掩码两份
  hdr.writeUInt16LE(1, 12);
  hdr.writeUInt16LE(32, 14);
  hdr.writeUInt32LE(0, 16); // BI_RGB 未压缩
  const xorLen = S * 4 * S;
  hdr.writeUInt32LE(xorLen, 20);
  // 掩码行按 1bpp 且 4 字节对齐
  const maskRow = Math.ceil(S / 32) * 4;
  const maskLen = maskRow * S;
  hdr.writeInt32LE(0, 24); hdr.writeInt32LE(0, 28);
  hdr.writeUInt32LE(0, 32); hdr.writeUInt32LE(0, 36);

  // BGRA，自下而上
  const xor = Buffer.alloc(xorLen);
  for (let y = 0; y < S; y++) {
    const srcRow = y * S * 4;
    const dstRow = (S - 1 - y) * S * 4;
    for (let x = 0; x < S; x++) {
      const i = srcRow + x * 4, o = dstRow + x * 4;
      xor[o] = rgba[i + 2];     // B
      xor[o + 1] = rgba[i + 1]; // G
      xor[o + 2] = rgba[i];     // R
      xor[o + 3] = rgba[i + 3]; // A
    }
  }
  // AND 掩码：全 0（透明交给 alpha；老系统按 alpha 处理 32bpp）
  const mask = Buffer.alloc(maskLen);
  return Buffer.concat([hdr, xor, mask]);
}

const frames = SIZES.map((S) => ({ S, data: dib(S, frame(S)) }));

const hdr = Buffer.alloc(6);
hdr.writeUInt16LE(0, 0); hdr.writeUInt16LE(1, 2); hdr.writeUInt16LE(frames.length, 4);
const dir = Buffer.alloc(frames.length * 16);
let off = 6 + frames.length * 16;
frames.forEach((fr, i) => {
  const b = i * 16;
  dir[b] = fr.S >= 256 ? 0 : fr.S;
  dir[b + 1] = fr.S >= 256 ? 0 : fr.S;
  dir.writeUInt16LE(1, b + 4);
  dir.writeUInt16LE(32, b + 6);
  dir.writeUInt32LE(fr.data.length, b + 8);
  dir.writeUInt32LE(off, b + 12);
  off += fr.data.length;
});
const ico = Buffer.concat([hdr, dir, ...frames.map((f) => f.data)]);

for (const o of process.argv.slice(2)) {
  fs.writeFileSync(o, ico);
  console.log('已写', o, ico.length, 'B 帧格式=未压缩位图 尺寸:', SIZES.join('/'));
}
