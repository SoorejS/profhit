# PROPHIT Deployment Guide

This guide describes how to deploy the PROPHIT application for a closed beta environment using Docker.

## Prerequisites
- A Linux-based server (e.g., Ubuntu 22.04 LTS on AWS EC2, DigitalOcean Droplet, or Linode)
- Docker and Docker Compose installed
- A registered domain name pointing to your server's IP
- An SSL proxy (like NGINX or Traefik) is highly recommended for production

## Deployment Steps

1. **Clone the Repository**
   ```bash
   git clone https://github.com/your-org/profhit.git
   cd profhit/backend
   ```

2. **Configure Environment Variables**
   Modify `docker-compose.yml` or use a `.env` file for your variables.
   **CRITICAL**: You MUST replace `JWT_SECRET` with a secure randomized string.
   ```bash
   openssl rand -hex 32
   ```
   Set `BETA_MODE=true` for testing without real limits.
   
   **Payment & Identity Integrations**:
   The application will fail to start if the following production keys are not provided:
   - `RAZORPAY_KEY_ID`: Your Razorpay key ID (Test/Live)
   - `RAZORPAY_KEY_SECRET`: Your Razorpay secret (Test/Live)
   - `RAZORPAY_WEBHOOK_SECRET`: Secret used to validate Razorpay webhooks
   - `HYPERVERGE_API_KEY`: Your HyperVerge App ID
   - `HYPERVERGE_API_SECRET`: Your HyperVerge App Key
   - `HYPERVERGE_WORKFLOW_ID`: The specific KYC workflow identifier
   - `HYPERVERGE_WEBHOOK_SECRET`: Secret used to validate HyperVerge webhooks
   - `HYPERVERGE_API_URL`: (Optional) Defaults to production `https://vrs.hyperverge.co/api/generateToken`. Can be overridden for sandbox.

3. **Start the Application**
   ```bash
   docker-compose up -d --build
   ```
   This command provisions:
   - A PostgreSQL 15 database running on `db:5432`
   - A Golang backend container running on port `8080`

4. **Verify Health**
   ```bash
   curl http://localhost:8080/api/health
   ```
   You should receive a `{"status": "ok"}` response.

5. **Static Frontend Setup**
   The frontend is purely static HTML/CSS/JS located in the root of the project.
   You can serve it via NGINX or a service like Vercel/Netlify.
   If using Vercel, simply connect the GitHub repository; it will automatically detect and deploy the static files.

## Database Migrations
Migrations are handled automatically by GORM upon application startup. No manual migration scripts are required.
To seed dummy data (optional for beta), ensure `config.SeedDatabase()` is enabled in `main.go`.
