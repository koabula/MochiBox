/**
 * Stream download utility to avoid loading entire files into memory
 * Supports both Electron and browser environments
 */

/**
 * Stream download a file from URL without loading it entirely into memory
 * @param url - URL to download from
 * @param filename - Filename for the downloaded file
 * @param onProgress - Optional progress callback (loaded, total)
 */
export async function streamDownload(
  url: string,
  filename: string,
  onProgress?: (loaded: number, total: number) => void
): Promise<void> {
  const response = await fetch(url)

  if (!response.ok) {
    throw new Error(`Download failed: ${response.statusText}`)
  }

  const totalSize = parseInt(response.headers.get('content-length') || '0')
  const reader = response.body?.getReader()

  if (!reader) {
    throw new Error('Response body is not readable')
  }

  // Check if running in Electron environment
  if (window.electron) {
    return streamDownloadElectron(reader, filename, totalSize, onProgress)
  } else {
    return streamDownloadBrowser(reader, filename, totalSize, onProgress)
  }
}

/**
 * Stream download for Electron environment using IPC
 */
async function streamDownloadElectron(
  reader: ReadableStreamDefaultReader<Uint8Array>,
  filename: string,
  totalSize: number,
  onProgress?: (loaded: number, total: number) => void
): Promise<void> {
  let downloaded = 0
  const chunks: Uint8Array[] = []
  const chunkSizeThreshold = 10 * 1024 * 1024 // 10MB threshold

  try {
    while (true) {
      const { done, value } = await reader.read()

      if (done) {
        // Write any remaining chunks
        if (chunks.length > 0) {
          const merged = mergeChunks(chunks)
          if (window.electron) {
            await window.electron.appendFile(filename, merged)
          }
        }
        break
      }

      if (value) {
        chunks.push(value)
        downloaded += value.length

        // Write to disk every 10MB to avoid memory buildup
        const bufferedSize = chunks.reduce((sum, chunk) => sum + chunk.length, 0)
        if (bufferedSize >= chunkSizeThreshold) {
          const merged = mergeChunks(chunks)
          if (window.electron) {
            await window.electron.appendFile(filename, merged)
          }
          chunks.length = 0 // Clear chunks after writing
        }

        // Update progress
        if (onProgress) {
          onProgress(downloaded, totalSize)
        }
      }
    }
  } finally {
    reader.releaseLock()
  }
}

/**
 * Stream download for browser environment (uses in-memory buffer)
 * Note: Browser environment still has memory constraints
 * For large files in browser, recommend using the backend download API instead
 */
async function streamDownloadBrowser(
  reader: ReadableStreamDefaultReader<Uint8Array>,
  filename: string,
  totalSize: number,
  onProgress?: (loaded: number, total: number) => void
): Promise<void> {
  let downloaded = 0
  const chunks: Uint8Array[] = []

  try {
    while (true) {
      const { done, value } = await reader.read()

      if (done) {
        break
      }

      if (value) {
        chunks.push(value)
        downloaded += value.length

        // Update progress
        if (onProgress) {
          onProgress(downloaded, totalSize)
        }
      }
    }

    // Merge all chunks and create blob
    const merged = mergeChunks(chunks)
    const blob = new Blob([merged.buffer as ArrayBuffer])

    // Trigger download
    const blobUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = blobUrl
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(blobUrl)
  } finally {
    reader.releaseLock()
  }
}

/**
 * Merge multiple Uint8Array chunks into a single Uint8Array
 */
function mergeChunks(chunks: Uint8Array[]): Uint8Array {
  const totalLength = chunks.reduce((sum, chunk) => sum + chunk.length, 0)
  const result = new Uint8Array(totalLength)
  let offset = 0

  for (const chunk of chunks) {
    result.set(chunk, offset)
    offset += chunk.length
  }

  return result
}

/**
 * Type declaration for window.electron (should match preload.ts)
 */
declare global {
  interface Window {
    electron?: {
      appendFile: (filename: string, data: Uint8Array) => Promise<void>
      // ... other electron APIs
    }
  }
}
