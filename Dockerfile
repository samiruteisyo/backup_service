FROM oven/bun:1

RUN apt-get update && apt-get install -y --no-install-recommends git && rm -rf /var/lib/apt/lists/* && git config --global --add safe.directory '*'

WORKDIR /app

COPY package.json bun.lockb* ./
RUN bun install --frozen-lockfile || bun install

COPY tsconfig.json ./
COPY src/ ./src/

CMD ["bun", "run", "src/index.ts"]
