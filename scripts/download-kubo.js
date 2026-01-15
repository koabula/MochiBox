const fs = require('fs');
const path = require('path');
const https = require('https');
const { execSync } = require('child_process');

const KUBO_VERSION = 'v0.26.0'; // 使用较新且稳定的版本
const PLATFORM = process.platform;
const ARCH = process.arch;

const OUTPUT_DIR = path.join(__dirname, '../electron/resources/bin');

if (!fs.existsSync(OUTPUT_DIR)) {
    fs.mkdirSync(OUTPUT_DIR, { recursive: true });
}

// 映射 Node.js 平台/架构到 Kubo 命名
const mapPlatform = (p) => {
    if (p === 'win32') return 'windows';
    return p;
}

const mapArch = (a) => {
    if (a === 'x64') return 'amd64';
    if (a === 'arm64') return 'arm64';
    return a;
}

const p = mapPlatform(PLATFORM);
const a = mapArch(ARCH);
const ext = p === 'windows' ? 'zip' : 'tar.gz';

const filename = `kubo_${KUBO_VERSION}_${p}-${a}.${ext}`;
const url = `https://dist.ipfs.tech/kubo/${KUBO_VERSION}/${filename}`;
const dest = path.join(OUTPUT_DIR, filename);

console.log(`Downloading Kubo ${KUBO_VERSION} for ${p}/${a}...`);
console.log(`URL: ${url}`);

const file = fs.createWriteStream(dest);
https.get(url, (response) => {
    if (response.statusCode !== 200) {
        console.error(`Failed to download: ${response.statusCode}`);
        process.exit(1);
    }

    response.pipe(file);

    file.on('finish', () => {
        file.close(() => {
            console.log('Download complete. Extracting...');
            
            try {
                if (p === 'windows') {
                    // 使用 PowerShell 解压
                    // Wait a bit for file handle release?
                    // But callback should be enough.
                    const cmd = `powershell -Command "Expand-Archive -Path '${dest}' -DestinationPath '${OUTPUT_DIR}' -Force"`;
                    execSync(cmd);
                
                // 移动文件: kubo/ipfs.exe -> ipfs.exe
                const src = path.join(OUTPUT_DIR, 'kubo', 'ipfs.exe');
                const target = path.join(OUTPUT_DIR, 'ipfs.exe');
                if (fs.existsSync(src)) {
                    fs.renameSync(src, target);
                    console.log(`Moved ${src} to ${target}`);
                } else {
                    console.error("ipfs.exe not found in extracted folder");
                }
            } else {
                // 使用 tar 解压
                execSync(`tar -xzf "${dest}" -C "${OUTPUT_DIR}"`);
                
                // 移动文件
                const src = path.join(OUTPUT_DIR, 'kubo', 'ipfs');
                const target = path.join(OUTPUT_DIR, 'ipfs');
                if (fs.existsSync(src)) {
                    fs.renameSync(src, target);
                    fs.chmodSync(target, 0o755); // 赋予执行权限
                    console.log(`Moved ${src} to ${target}`);
                }
            }

            // 清理
            fs.unlinkSync(dest);
            const kuboDir = path.join(OUTPUT_DIR, 'kubo');
            if (fs.existsSync(kuboDir)) {
                fs.rmSync(kuboDir, { recursive: true, force: true });
            }
            
            console.log('Kubo setup complete!');

        } catch (err) {
            console.error('Extraction failed:', err);
            process.exit(1);
        }
    });
});
}).on('error', (err) => {
    fs.unlink(dest, () => {});
    console.error('Download error:', err.message);
    process.exit(1);
});
