// 零依赖读 PNG（标准库 zlib），按坐标报像素色。用来验「存出来的图里箭头到底在不在」。
// 用法：node tools/peek-pixel.cjs <png> <x,y> [x,y ...]
const fs = require('node:fs');
const zlib = require('node:zlib');

function decodePNG(buf) {
  let p = 8, w = 0, h = 0, depth = 0, ctype = 0;
  const idat = [];
  while (p < buf.length) {
    const len = buf.readUInt32BE(p);
    const type = buf.toString('ascii', p + 4, p + 8);
    const data = buf.subarray(p + 8, p + 8 + len);
    if (type === 'IHDR') {
      w = data.readUInt32BE(0); h = data.readUInt32BE(4);
      depth = data[8]; ctype = data[9];
    } else if (type === 'IDAT') idat.push(data);
    else if (type === 'IEND') break;
    p += 12 + len;
  }
  if (depth !== 8) throw new Error('只支持 8 位深，实际 ' + depth);
  const ch = ctype === 6 ? 4 : ctype === 2 ? 3 : ctype === 0 ? 1 : (() => { throw new Error('不支持的色彩类型 ' + ctype); })();
  const raw = zlib.inflateSync(Buffer.concat(idat));
  const stride = w * ch;
  const out = Buffer.alloc(h * stride);
  const paeth = (a, b, c) => {
    const pa = Math.abs(b - c), pb = Math.abs(a - c), pc = Math.abs(a + b - 2 * c);
    return pa <= pb && pa <= pc ? a : pb <= pc ? b : c;
  };
  for (let y = 0; y < h; y++) {
    const f = raw[y * (stride + 1)];
    const line = raw.subarray(y * (stride + 1) + 1, y * (stride + 1) + 1 + stride);
    const cur = out.subarray(y * stride, (y + 1) * stride);
    const prev = y ? out.subarray((y - 1) * stride, y * stride) : Buffer.alloc(stride);
    for (let x = 0; x < stride; x++) {
      const a = x >= ch ? cur[x - ch] : 0, b = prev[x], c = x >= ch ? prev[x - ch] : 0;
      let v = line[x];
      if (f === 1) v = (v + a) & 255;
      else if (f === 2) v = (v + b) & 255;
      else if (f === 3) v = (v + ((a + b) >> 1)) & 255;
      else if (f === 4) v = (v + paeth(a, b, c)) & 255;
      cur[x] = v;
    }
  }
  return { w, h, ch, px: out };
}

const file = process.argv[2];
const img = decodePNG(fs.readFileSync(file));
const hex = (x, y) => {
  const i = (Math.round(y) * img.w + Math.round(x)) * img.ch;
  const h = (n) => n.toString(16).padStart(2, '0');
  return `#${h(img.px[i])}${h(img.px[i + 1])}${h(img.px[i + 2])}`;
};
console.log(`${file} 尺寸=${img.w}x${img.h} 通道=${img.ch}`);
for (const arg of process.argv.slice(3)) {
  const [x, y] = arg.split(',').map(Number);
  console.log(`  (${x},${y}) = ${hex(x, y)}`);
}
