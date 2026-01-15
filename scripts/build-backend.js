const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const BACKEND_DIR = path.join(__dirname, '../backend');
const OUTPUT_BASE = path.join(__dirname, '../electron/resources/bin');

const targets = [
    { os: 'windows', arch: 'amd64', folder: 'win', name: 'mochibox-core.exe' },
    { os: 'linux', arch: 'amd64', folder: 'linux', name: 'mochibox-core' },
    { os: 'darwin', arch: 'amd64', folder: 'mac', name: 'mochibox-core' }
];

console.log('Building Go Backend for multiple platforms...');

targets.forEach(target => {
    const outputDir = path.join(OUTPUT_BASE, target.folder);
    const outputPath = path.join(outputDir, target.name);

    if (!fs.existsSync(outputDir)) {
        fs.mkdirSync(outputDir, { recursive: true });
    }

    console.log(`Building for ${target.os}/${target.arch}...`);

    const env = { 
        ...process.env, 
        CGO_ENABLED: '0',
        GOOS: target.os, 
        GOARCH: target.arch 
    };

    try {
        execSync(`go build -o "${outputPath}"`, { 
            cwd: BACKEND_DIR,
            env: env,
            stdio: 'inherit'
        });
        console.log(`Success: ${outputPath}`);
    } catch (err) {
        console.error(`Failed to build for ${target.os}:`, err);
        process.exit(1);
    }
});

console.log('All backend builds complete!');
