FROM node:22-alpine AS builder

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@latest --activate

ENV NEW_RELIC_NO_CONFIG_FILE=true
ENV NEW_RELIC_DISTRIBUTED_TRACING_ENABLED=true
ENV NEW_RELIC_LOG=stdout

COPY pnpm-lock.yaml* package.json ./
RUN pnpm install --frozen-lockfile --config.dangerouslyAllowAllBuilds=true

COPY . .

RUN pnpm build

FROM nginx:alpine AS runner 

COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 3001

CMD ["nginx", "-g", "daemon off;"]
