---
sidebar_position: 4
---

# Configuration

The Tekmetric MCP Server can be configured using environment variables, configuration files, or a combination of both.

## Configuration Methods

### 1. Environment Variables (Recommended)

Set environment variables in your shell or Claude Desktop config:

```bash
export TEKMETRIC_CLIENT_ID="your_client_id"
export TEKMETRIC_CLIENT_SECRET="your_client_secret"
export TEKMETRIC_BASE_URL="https://shop.tekmetric.com"
export TEKMETRIC_DEFAULT_SHOP_ID="123"
export TEKMETRIC_DEBUG="false"
```

### 2. Configuration File

Create `~/.config/tekmetric-mcp/config.json`:

```json
{
  "tekmetric": {
    "base_url": "https://shop.tekmetric.com",
    "client_id": "your_client_id",
    "client_secret": "your_client_secret",
    "default_shop_id": 123,
    "timeout_seconds": 30,
    "max_retries": 3,
    "max_backoff_sec": 60
  },
  "server": {
    "name": "tekmetric-mcp",
    "version": "0.1.0",
    "debug": false
  }
}
```

### 3. Command Line Flags

Override configuration via command line:

```bash
tekmetric-mcp serve \
  --client-id=your_id \
  --client-secret=your_secret \
  --base-url=https://shop.tekmetric.com \
  --shop-id=123
```

## Configuration Precedence

Configuration is loaded in this order (highest priority first):

1. **Command line flags** - Override everything
2. **Environment variables** - Override config file
3. **Configuration file** - Base configuration
4. **Default values** - Fallback values

## Configuration Options

### Tekmetric API Settings

#### `TEKMETRIC_CLIENT_ID` (required)

OAuth2 client ID from Tekmetric API Access settings.

- **Type**: string
- **Required**: Yes
- **Example**: `"abc123def456"`

#### `TEKMETRIC_CLIENT_SECRET` (required)

OAuth2 client secret from Tekmetric API Access settings.

- **Type**: string
- **Required**: Yes
- **Example**: `"secret_xyz789"`
- **Security**: Never commit this value

#### `TEKMETRIC_BASE_URL`

Tekmetric API base URL.

- **Type**: string
- **Required**: No
- **Default**: `https://sandbox.tekmetric.com`
- **Options**:
  - `https://sandbox.tekmetric.com` - Testing
  - `https://shop.tekmetric.com` - Production

#### `TEKMETRIC_DEFAULT_SHOP_ID`

Default shop ID for API calls.

- **Type**: integer
- **Required**: No
- **Default**: 0 (must be specified in tool calls)
- **Example**: `123`

#### `TEKMETRIC_TIMEOUT_SECONDS`

HTTP request timeout in seconds.

- **Type**: integer
- **Required**: No
- **Default**: 30
- **Range**: 1-300

#### `TEKMETRIC_MAX_RETRIES`

Maximum retry attempts for failed requests.

- **Type**: integer
- **Required**: No
- **Default**: 3
- **Range**: 0-10
- **Note**: 0 disables retries

#### `TEKMETRIC_MAX_BACKOFF_SEC`

Maximum backoff time in seconds between retries.

- **Type**: integer
- **Required**: No
- **Default**: 60
- **Range**: 1-300
- **Formula**: min(2^n + jitter, max_backoff)

### Server Settings

#### `TEKMETRIC_DEBUG`

Enable debug logging.

- **Type**: boolean
- **Required**: No
- **Default**: false
- **Values**: `true`, `false`

#### `TEKMETRIC_TRANSPORT`

The transport the server uses to talk to a client.

- **Type**: string
- **Required**: No
- **Default**: `stdio`
- **Values**:
  - `stdio` - One client over standard input and output. Claude Desktop uses this transport.
  - `http` - Many clients over streamable HTTP at the `/mcp` path.

#### `TEKMETRIC_ADDR`

The listen address for the `http` transport.

- **Type**: string
- **Required**: No
- **Default**: `:8080` (all interfaces)
- **Example**: `127.0.0.1:8080`

#### `TEKMETRIC_HTTP_TOKEN`

The bearer token that each `http` request must send in the `Authorization` header.

- **Type**: string
- **Required**: Yes, for the `http` transport
- **Default**: none
- **Example request header**: `Authorization: Bearer <token>`
- **Security**: Never commit this value

The server uses the shop's Tekmetric credentials for every tool call. A caller that reaches the port can read customer names, emails, phone numbers, and addresses. For this reason, the `http` transport does not start without a token.

Make a token with high entropy:

```bash
export TEKMETRIC_HTTP_TOKEN="$(openssl rand -hex 32)"
```

The server does not serve TLS. Put a TLS proxy in front of the server when clients connect over a network, so that the token does not cross the network in clear text.

## Claude Desktop Configuration

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "tekmetric": {
      "command": "/absolute/path/to/tekmetric-mcp",
      "args": ["serve"],
      "env": {
        "TEKMETRIC_CLIENT_ID": "your_client_id",
        "TEKMETRIC_CLIENT_SECRET": "your_client_secret",
        "TEKMETRIC_BASE_URL": "https://shop.tekmetric.com",
        "TEKMETRIC_DEFAULT_SHOP_ID": "123"
      }
    }
  }
}
```

**Important**:
- Use absolute paths (not `~` or relative paths)
- Restart Claude Desktop after changes
- Check for JSON syntax errors

## Environment-Specific Configuration

### Development/Testing

```bash
TEKMETRIC_BASE_URL=https://sandbox.tekmetric.com
TEKMETRIC_DEFAULT_SHOP_ID=2
TEKMETRIC_DEBUG=true
TEKMETRIC_MAX_RETRIES=1
```

### Production

```bash
TEKMETRIC_BASE_URL=https://shop.tekmetric.com
TEKMETRIC_DEFAULT_SHOP_ID=123
TEKMETRIC_DEBUG=false
TEKMETRIC_MAX_RETRIES=3
TEKMETRIC_TIMEOUT_SECONDS=30
```

## Security Best Practices

### Credential Management

1. **Never commit credentials** - Use `.env` files (in `.gitignore`)
2. **Separate environments** - Different credentials for sandbox/production
3. **Rotate regularly** - Update credentials periodically
4. **Use secrets managers** - Consider AWS Secrets Manager, HashiCorp Vault, etc.
5. **Minimal permissions** - Only grant necessary shop access

### Configuration Files

```bash
# Secure your config file
chmod 600 ~/.config/tekmetric-mcp/config.json

# Check permissions
ls -la ~/.config/tekmetric-mcp/config.json
```

Should show: `-rw------- (600)`

## Troubleshooting

### Configuration Not Loading

1. **Check file location**:
```bash
ls -la ~/.config/tekmetric-mcp/config.json
```

2. **Validate JSON syntax**:
```bash
cat ~/.config/tekmetric-mcp/config.json | python -m json.tool
```

3. **Check environment variables**:
```bash
env | grep TEKMETRIC
```

### Debug Mode

Enable debug logging to see configuration details:

```bash
TEKMETRIC_DEBUG=true ./tekmetric-mcp serve
```

Or use the `-d` flag:

```bash
./tekmetric-mcp -d serve
```

## Next Steps

- See [Usage Examples](../examples/index.md)
- Review [Available Tools](../tools/index.md)
- Check [Installation Guide](../installation/index.md)
