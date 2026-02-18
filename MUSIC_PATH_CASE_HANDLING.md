# Music File Path Case Handling

## Overview
This document explains how Athena handles music file paths and the considerations for webAO compatibility.

## How It Works

### Server Behavior
1. **Music List Loading**: The server reads `music.txt` and preserves the original case of all entries (both category names and file paths).
2. **Display**: The server sends the music list to clients with the original case intact, so it displays exactly as written in `music.txt`.
3. **Validation**: When a client requests to play music, the server validates the request using **case-insensitive** comparison against the music list.

### Example
If your `music.txt` contains:
```
Prelude
Ace Attorney/Prelude/[AA] Opening.opus
```

- The server will send `"Ace Attorney/Prelude/[AA] Opening.opus"` to clients (preserving capital letters)
- If a client sends `"ace attorney/prelude/[aa] opening.opus"` (all lowercase), the server will still accept it
- The music list display will show: **"Ace Attorney/Prelude/[AA] Opening.opus"** (original case)

## Important Limitation: WebAO File Loading

⚠️ **Critical**: The server's case-insensitive validation does NOT solve the webAO file loading problem!

### The Problem
When a webAO client tries to play music:
1. The server broadcasts the music change packet with the path from `music.txt` (e.g., `"Ace Attorney/Prelude/[AA] Opening.opus"`)
2. Each client's **browser** attempts to load: `webao_base_url + "Ace Attorney/Prelude/[AA] Opening.opus"`
3. If the actual file is `ace attorney/prelude/[aa] opening.opus` (lowercase), the browser's HTTP request fails with a 404 error
4. The music doesn't play, and clients may crash or disconnect

### Why This Happens
- HTTP URLs are **case-sensitive** on most web servers (Linux/Unix-based)
- The **browser** (not the Athena server) makes the file loading request
- Athena has no control over how the browser loads files from the webAO base URL

## Solutions

You have three options to fix the webAO file loading issue:

### Option 1: Rename Your Files (Recommended)
Rename all your music files to match the exact case in `music.txt`.

**Example:**
```bash
# If music.txt has: Ace Attorney/Prelude/[AA] Opening.opus
# Rename the file to match:
mv "ace attorney/prelude/[aa] opening.opus" "Ace Attorney/Prelude/[AA] Opening.opus"
```

✅ Pros: Simple, works immediately, display looks nice
❌ Cons: Requires manual file renaming

### Option 2: Configure Your Web Server to Be Case-Insensitive
If you control the webAO base URL web server, configure it to serve files case-insensitively.

**For nginx:**
```nginx
location /sounds/ {
    # Use a Lua script or URL rewriting to handle case-insensitive paths
    # This is complex and not recommended
}
```

**For Apache:**
```apache
# Enable mod_speling (note the intentional misspelling)
CheckSpelling On
CheckCaseOnly On
```

✅ Pros: No file renaming needed, works for all files
❌ Cons: Requires web server access and configuration, may have performance impact

### Option 3: Use Lowercase in music.txt (Not Recommended)
Edit `music.txt` to use all lowercase paths to match your files.

**Example:**
```
Prelude
ace attorney/prelude/[aa] opening.opus
```

✅ Pros: Files load correctly
❌ Cons: Display looks ugly (all lowercase)

## Recommendation

**Use Option 1** (rename your files to match `music.txt`). This gives you:
- Beautiful, properly-formatted display names
- Files that load correctly
- No web server configuration needed

## Testing Your Setup

To test if your music files will load correctly:
1. Check the case of your music paths in `music.txt`
2. Check the actual file names on your webAO server
3. Ensure they match EXACTLY (including case)
4. Try loading music in a client - it should work without errors

## Technical Details

### What the Server-Side Case-Insensitive Validation Does
- Allows clients to send music change requests with any case variant
- Prevents false "invalid music" errors due to case mismatches
- Makes the server more permissive and user-friendly

### What It Does NOT Do
- Does NOT change how files are sent to clients
- Does NOT affect browser file loading behavior  
- Does NOT solve the webAO file loading problem

The server-side case-insensitive validation is a quality-of-life improvement for server-side validation, but the fundamental file loading issue must be solved at the file system or web server level.
