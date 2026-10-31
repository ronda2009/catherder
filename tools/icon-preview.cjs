// 把图标画成 BMP 预览（BMP 谁都能解码，便于肉眼检查图案）
// 用法: node icon-preview.cjs <尺寸> <输出.bmp> [背景灰度 0-255]
const fs = require('fs');

const src = fs.readFileSync(__dirname + '\\genicon2.cjs', 'utf8');
const body = src.slice(src.indexOf('function frame(S)'), src.indexOf('const frames ='));
const frame = new Function('S', body + '\nreturn frame(S);');

function bmp(S, rgba, bg, out) {
  const row = S * 3;
  const pad = (4 - (row % 4)) % 4;
  const dataLen = (row + pad) * S;
  const fh = Buffer.alloc(14);
  fh.write('BM', 0);
  fh.writeUInt32LE(14 + 40 + dataLen, 2);
  fh.writeUInt32LE(54, 10);
  const ih = Buffer.alloc(40);
  ih.writeUInt32LE(40, 0);
  ih.writeInt32LE(S, 4); ih.writeInt32LE(-S, 8); // 负高 = 从上往下
  ih.writeUInt16LE(1, 12); ih.writeUInt16LE(24, 14);
  ih.writeUInt32LE(dataLen, 20);
  const px = Buffer.alloc(dataLen);
  for (let y = 0; y < S; y++) for (let x = 0; x < S; x++) {
    const i = (y * S + x) * 4, o = y * (row + pad) + x * 3;
    const a = rgba[i + 3] / 255;
    px[o + 2] = Math.round(rgba[i] * a + bg * (1 - a));
    px[o + 1] = Math.round(rgba[i + 1] * a + bg * (1 - a));
    px[o] = Math.round(rgba[i + 2] * a + bg * (1 - a));
  }
  fs.writeFileSync(out, Buffer.concat([fh, ih, px]));
  console.log('已写', out, S + 'x' + S, '背景灰度', bg);
}

const S = parseInt(process.argv[2] || '96', 10);
const out = process.argv[3] || 'preview.bmp';
const bg = parseInt(process.argv[4] || '255', 10);
bmp(S, frame(S), bg, out);
