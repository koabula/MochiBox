/**
 * Unified clipboard utility with fallback mechanisms
 * Handles document focus issues in Electron and web environments
 */

export async function copyToClipboard(text: string): Promise<boolean> {
    if (!text) {
        return false;
    }

    // Method 1: Try Electron clipboard API (most reliable in Electron)
    try {
        // @ts-ignore
        if (window.electronAPI?.clipboard?.writeText) {
            // @ts-ignore
            await window.electronAPI.clipboard.writeText(text);
            return true;
        }
    } catch (e) {
        console.warn('Electron clipboard failed, trying web API', e);
    }

    // Method 2: Try Web Clipboard API
    if (navigator.clipboard?.writeText) {
        try {
            await navigator.clipboard.writeText(text);
            return true;
        } catch (e) {
            console.warn('Web clipboard failed, trying fallback', e);
        }
    }

    // Method 3: Fallback to execCommand (works without focus in some cases)
    try {
        const textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        textarea.style.top = '-9999px';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        
        textarea.focus();
        textarea.select();
        
        try {
            textarea.setSelectionRange(0, text.length);
        } catch (e) {
            // Some older browsers don't support setSelectionRange
        }
        
        const success = document.execCommand('copy');
        document.body.removeChild(textarea);
        
        if (success) {
            return true;
        }
    } catch (e) {
        console.error('execCommand copy failed', e);
    }

    return false;
}
