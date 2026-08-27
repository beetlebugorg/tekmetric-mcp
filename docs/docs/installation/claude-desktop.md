---
sidebar_position: 1
---

# Claude Desktop Installation

**The easiest way to get started!** This guide shows you how to install the Tekmetric assistant using Claude Desktop's one-click installer.

## What You'll Need

- [Claude Desktop app](https://claude.ai/download) installed
- Your Tekmetric API credentials ([see here](../installation/index.md#tekmetric-api-credentials))

## Step 1: Download the Desktop Extension

1. Go to [Releases](https://github.com/beetlebugorg/tekmetric-mcp/releases)
2. Download the `.mcpb` file, named `tekmetric-mcp-<version>.mcpb`
3. Save it somewhere you can find it (like your Downloads folder)

## Step 2: Install the Extension

1. **Double-click** the `.mcpb` file you downloaded
2. Claude Desktop will automatically open
3. You'll see a prompt asking you to configure the server

That's it! The file is installed automatically.

## Step 3: Configure Your Credentials

Claude Desktop will ask you for these values:

### Required Fields

**TEKMETRIC_CLIENT_ID**
- Paste your Client ID from Tekmetric
- This is the username for API access

**TEKMETRIC_CLIENT_SECRET**
- Paste your Client Secret from Tekmetric
- This is the password for API access
- ⚠️ Keep this secret!

**TEKMETRIC_BASE_URL**
- For production (real data): `https://shop.tekmetric.com`
- For testing (sandbox): `https://sandbox.tekmetric.com`

**TEKMETRIC_DEFAULT_SHOP_ID**
- Your shop ID number (like `123`)
- Find this in Tekmetric Settings or ask your account manager
- For sandbox testing, use `2` or `3`

### Save Configuration

After entering all fields:
1. Click **Save** or **Apply**
2. Claude Desktop will automatically restart the connection

## Step 4: Verify It's Working

Look for the **🔌 icon** in the bottom-right corner of Claude Desktop. This means the server is connected!

Try asking Claude:
```
Show me my shops
```

If you see your shop information, **you're all set!** 🎉

## Troubleshooting

### ❌ Can't find the .mcpb file

**Solution**: Check your Downloads folder or wherever your browser saves files

### ❌ "Authentication failed"

**Problem**: Your credentials aren't working

**Solutions**:
- Double-check you copied the Client ID and Client Secret correctly
- Make sure there are no extra spaces
- Make sure you're using credentials for the right Tekmetric account
- Try regenerating credentials in Tekmetric

### ❌ "Shop not found"

**Problem**: Wrong shop ID

**Solutions**:
- Check your shop ID in Tekmetric Settings
- For sandbox, try `2` or `3`
- Ask Claude "Show me all my shops" to see available IDs

### ❌ No 🔌 icon appears

**Problem**: Server didn't start

**Solutions**:
1. Completely quit Claude Desktop (⌘+Q on Mac, Alt+F4 on Windows)
2. Reopen Claude Desktop
3. Look for the 🔌 icon again
4. Check Claude Desktop logs: Help → View Logs

### ❌ Extension won't install

**Problem**: Mac security blocking the file

**Solutions**:
1. Right-click the .mcpb file
2. Select "Open With" → Claude Desktop
3. Click "Open" when warned about unidentified developer

## Updating to a New Version

When a new version is released:

1. Download the new `.mcpb` file
2. Double-click it to install
3. Your existing configuration will be preserved
4. Restart Claude Desktop

## Uninstalling

To remove the Tekmetric assistant:

### On Mac

1. Open Claude Desktop
2. Go to Settings → Extensions
3. Find "tekmetric-mcp"
4. Click Remove

### On Windows

1. Open Claude Desktop
2. Go to Settings → Extensions
3. Find "tekmetric-mcp"
4. Click Remove

## Next Steps

✅ **Installation complete!** Now you can:

- Try [Usage Examples](../examples/index.md) to see what questions to ask
- Learn about [Available Tools](../tools/index.md)
- Start asking Claude about your shop data!

## Security Notes

Your API credentials are stored securely in Claude Desktop's configuration:
- Mac: `~/Library/Application Support/Claude/`
- Windows: `%APPDATA%\Claude\`

Only Claude Desktop can access these credentials. They're never sent anywhere except directly to Tekmetric's API.
