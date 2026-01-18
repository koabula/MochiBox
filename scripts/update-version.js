const fs = require('fs');
const path = require('path');

const newVersion = process.argv[2];

if (!newVersion) {
  console.error('Please provide a version number. Usage: node scripts/update-version.js <version>');
  process.exit(1);
}

// Define the files to update
const files = [
  path.join(__dirname, '..', 'package.json'),
  path.join(__dirname, '..', 'electron', 'package.json'),
  path.join(__dirname, '..', 'frontend', 'package.json')
];

console.log(`Setting version to: ${newVersion}`);

files.forEach(file => {
  if (fs.existsSync(file)) {
    try {
      const content = fs.readFileSync(file, 'utf8');
      const json = JSON.parse(content);
      const oldVersion = json.version;
      
      json.version = newVersion;
      
      // Write back with 2 spaces indentation and a trailing newline
      fs.writeFileSync(file, JSON.stringify(json, null, 2) + '\n');
      console.log(`Updated ${path.relative(path.join(__dirname, '..'), file)}: ${oldVersion} -> ${newVersion}`);
    } catch (err) {
      console.error(`Error updating ${file}:`, err.message);
    }
  } else {
    console.warn(`File not found: ${file}`);
  }
});

console.log('Version update complete.');
