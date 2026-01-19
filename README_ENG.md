# MochiBox

[中文](./README.md) | [English](./README_ENG.md)

> A secure, decentralized file sharing tool powered by IPFS.

MochiBox is a decentralized file sharing tool built on IPFS, designed to provide a secure, private, and easy-to-use file transfer experience.

![alt text](./resources/image.png)

## Features

- **Decentralized Storage**: Based on IPFS technology, file sharding storage, no centralized server.
- **End-to-End Encryption**: Supports end-to-end encryption mode, ensuring files are visible only to the recipient.
- **Cross-Platform**: Supports Windows, macOS, and Linux.

## Installation Instructions

Download the installation package for your system directly from the [GitHub Releases](https://github.com/koabula/MochiBox/releases) page.

## How to Use it

### Init MochiBox

1. After launching MochiBox for the first time, please select **Create New Account**. This will generate a new key pair (wallet) locally, which will serve as your unique user identity. Please keep your mnemonic phrase safe, as it is the only way to recover your account.
2. Afterward, you need to set a password, which is used to protect your local data and for authentication when opening MochiBox.
3. MochiBox is based on IPFS. Please enable **Use Built-in IPFS Node** in **Settings**, which will automatically start the IPFS node. Alternatively, you can manually configure your own IPFS node address.
4. After completing the above steps, you can start using MochiBox.

### Upload a File

Click the **Upload** button in the top right corner of MochiBox to upload a file. currently, there are three modes:
1. **Public**: All users can view and download the file.
2. **Password**: You need to enter a password, and only users who know the password can view and download the file.
3. **Private**: You need to enter a user ID (viewable in the other party's Account interface, which is essentially their public key), and only that user can view and download the file.

After a successful upload, you can view the uploaded file in **My Files**, and you can preview, download, share, and delete it. Note that Private mode files can only be viewed if you are the intended recipient.

Private mode is end-to-end encrypted (E2EE). simply put, only you and the recipient can decrypt and view the file content throughout the entire process. Other users cannot decrypt and view the file content even if they know the file's IPFS CID.

### Get a File

In **My Files**, you can generate a share link for the file (Mochi Link), which is a URL starting with "mochi://".

If you receive a Mochi Link, you can enter it in the **Shared** interface. Then you can preview, download, or Pin the file. (In Password mode, you need to enter the correct password).

**Pin** refers to adding the file to your IPFS node so that you can download it via the IPFS protocol at any time, even if the sharer is offline.

## Build Instructions

This project uses a Go backend + Electron frontend. Before building, please ensure that **Node.js (v20+)** and **Go (v1.21+)** are installed.

1. **Install Dependencies**
   ```bash
   npm install
   ```

2. **Build Backend and Frontend**
   
   Run the build command in the root directory:
   ```bash
   npm run build
   ```
   
   This command will automatically perform the following steps:
   - Download IPFS binary dependencies
   - Compile Go backend core
   - Build Vue frontend resources
   - Move resources to the Electron directory

3. **Package for Release**

   Enter the `electron` directory and run the packaging command according to the target platform:

   ```bash
   cd electron
   
   # Windows (generates .exe)
   npm run dist -- --win
   
   # Linux (generates .AppImage, .deb)
   npm run dist -- --linux
   
   # macOS (generates .dmg)
   npm run dist -- --mac
   ```

4. **Output Artifacts**
   After packaging is complete, the installation package will be located in the `electron/dist_electron` directory.

## License

This project is open-sourced under the [GPL-3.0](LICENSE) license.
