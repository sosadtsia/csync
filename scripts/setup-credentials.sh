#!/bin/bash

# csync Secure Credentials Setup Script
# This script helps you set up environment variables for secure credential storage

echo "🔐 csync Secure Credentials Setup"
echo "=================================="
echo ""
echo "This script will help you set up environment variables for secure credential storage."
echo "Your credentials will NOT be stored in configuration files."
echo ""

# Create .env file for local development
ENV_FILE=".env"

echo "Creating $ENV_FILE file for local development..."
echo ""

# pCloud credentials
echo "📁 pCloud Configuration:"
read -p "Enter your pCloud username/email: " PCLOUD_USERNAME
read -s -p "Enter your pCloud password: " PCLOUD_PASSWORD
echo ""

# Add to .env file
cat > "$ENV_FILE" << EOF
# pCloud Credentials
export PCLOUD_USERNAME="$PCLOUD_USERNAME"
export PCLOUD_PASSWORD="$PCLOUD_PASSWORD"

# Google Drive Credentials (optional - override paths)
# export GOOGLE_CREDENTIALS_PATH="/path/to/your/credentials.json"
# export GOOGLE_TOKEN_PATH="/path/to/your/token.json"
EOF

echo ""
echo "✅ Created $ENV_FILE file"
echo ""
echo "🚀 Usage Instructions:"
echo "====================="
echo ""
echo "1. Source the environment file before running csync:"
echo "   source .env"
echo "   ./csync -config examples/csync-secure.json -provider pcloud"
echo ""
echo "2. Or export variables in your shell profile (~/.bashrc, ~/.zshrc):"
echo "   echo 'export PCLOUD_USERNAME=\"$PCLOUD_USERNAME\"' >> ~/.bashrc"
echo "   echo 'export PCLOUD_PASSWORD=\"$PCLOUD_PASSWORD\"' >> ~/.bashrc"
echo ""
echo "3. For production/server deployment, set environment variables:"
echo "   export PCLOUD_USERNAME=\"your-username\""
echo "   export PCLOUD_PASSWORD=\"your-password\""
echo ""
echo "🔒 Security Notes:"
echo "=================="
echo "• Never commit .env files to version control"
echo "• Use secure secret management in production (AWS Secrets Manager, etc.)"
echo "• Consider using OAuth2 for Google Drive instead of service account keys"
echo "• Rotate credentials regularly"
