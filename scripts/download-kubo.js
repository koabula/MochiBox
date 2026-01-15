const fs = require('fs');
const path = require('path');
const https = require('https');
const { execSync } = require('child_process');

const KUBO_VERSION = 'v0.26.0';
const BASE_OUTPUT_DIR = path.join(__dirname, '../electron/resources/bin');

// Define targets
const targets = [
    { platform: 'windows', arch: 'amd64', folder: 'win', ext: 'zip', binary: 'ipfs.exe' },
    { platform: 'linux', arch: 'amd64', folder: 'linux', ext: 'tar.gz', binary: 'ipfs' },
    { platform: 'darwin', arch: 'amd64', folder: 'mac', ext: 'tar.gz', binary: 'ipfs' }
];

// Ensure base dir exists
if (!fs.existsSync(BASE_OUTPUT_DIR)) {
    fs.mkdirSync(BASE_OUTPUT_DIR, { recursive: true });
}

async function downloadFile(url, dest) {
    return new Promise((resolve, reject) => {
        const file = fs.createWriteStream(dest);
        https.get(url, (response) => {
            if (response.statusCode !== 200) {
                reject(new Error(`Failed to download: ${response.statusCode}`));
                return;
            }
            response.pipe(file);
            file.on('finish', () => {
                file.close(() => resolve());
            });
        }).on('error', (err) => {
            fs.unlink(dest, () => {});
            reject(err);
        });
    });
}

function extractAndMove(archivePath, targetFolder, binaryName, isZip) {
    const tempDir = path.join(targetFolder, 'temp_extract');
    if (!fs.existsSync(tempDir)) fs.mkdirSync(tempDir, { recursive: true });

    console.log(`Extracting ${path.basename(archivePath)}...`);

    try {
        if (isZip) {
            // Use PowerShell for zip on Windows
            const cmd = `powershell -Command "$ProgressPreference = 'SilentlyContinue'; Expand-Archive -Path '${archivePath}' -DestinationPath '${tempDir}' -Force"`;
            execSync(cmd);
        } else {
            // Use tar for tar.gz (Windows 10+ has tar)
            // If tar is not available, this might fail on old Windows. Assuming Dev env has git bash or modern windows.
            try {
                execSync(`tar -xzf "${archivePath}" -C "${tempDir}"`);
            } catch (e) {
                console.log("tar failed, trying to use 7z if available or warn user.");
                throw e;
            }
        }

        // Find binary in kubo/ (it extracts to a 'kubo' folder usually)
        const possiblePaths = [
            path.join(tempDir, 'kubo', binaryName),
            path.join(tempDir, binaryName)
        ];

        let src = possiblePaths.find(p => fs.existsSync(p));
        if (src) {
            const dest = path.join(targetFolder, binaryName);
            // Remove existing if any
            if (fs.existsSync(dest)) fs.unlinkSync(dest);
            
            fs.renameSync(src, dest);
            
            // Chmod +x for non-windows
            if (!binaryName.endsWith('.exe')) {
                fs.chmodSync(dest, 0o755);
            }
            console.log(`Installed to ${dest}`);
        } else {
            throw new Error(`Binary ${binaryName} not found in archive.`);
        }

    } finally {
        // Cleanup temp
        try {
            if (fs.existsSync(tempDir)) fs.rmSync(tempDir, { recursive: true, force: true });
            if (fs.existsSync(archivePath)) fs.unlinkSync(archivePath);
        } catch (e) {
            console.warn("Cleanup warning:", e.message);
        }
    }
}

async function processTarget(target) {
    const targetDir = path.join(BASE_OUTPUT_DIR, target.folder);
    if (!fs.existsSync(targetDir)) {
        fs.mkdirSync(targetDir, { recursive: true });
    }

    const filename = `kubo_${KUBO_VERSION}_${target.platform}-${target.arch}.${target.ext}`;
    const url = `https://dist.ipfs.tech/kubo/${KUBO_VERSION}/${filename}`;
    const dest = path.join(targetDir, filename);

    console.log(`Processing ${target.platform}/${target.arch}...`);
    
    // Check if binary already exists to skip? (Optional, but good for speed)
    // if (fs.existsSync(path.join(targetDir, target.binary))) {
    //     console.log("Binary already exists, skipping download.");
    //     return;
    // }

    await downloadFile(url, dest);
    extractAndMove(dest, targetDir, target.binary, target.ext === 'zip');
}

async function main() {
    console.log("Starting multi-platform Kubo download...");
    for (const target of targets) {
        try {
            await processTarget(target);
        } catch (err) {
            console.error(`Error processing ${target.platform}:`, err);
            process.exit(1);
        }
    }
    console.log("All Kubo binaries downloaded successfully!");
}

main();
