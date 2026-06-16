# Base stage
FROM node:20-alpine AS base

# Dependencies stage
FROM base AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install

# Builder stage
FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Declare build arguments and environment variables for NEXT_PUBLIC_ vars
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_LOGO_DEV_TOKEN
ENV NEXT_PUBLIC_LOGO_DEV_TOKEN=$NEXT_PUBLIC_LOGO_DEV_TOKEN
ARG ACTIVE_THRESHOLD_DAYS
ENV ACTIVE_THRESHOLD_DAYS=$ACTIVE_THRESHOLD_DAYS

# Disable telemetry
ENV NEXT_TELEMETRY_DISABLED=1

RUN npm run build

# Runner stage
FROM base AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

# Copy the standalone output and public files
COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000
ENV PORT=3000

CMD ["node", "server.js"]
