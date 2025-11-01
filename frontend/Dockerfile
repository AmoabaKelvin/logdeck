FROM oven/bun:latest

WORKDIR /app

COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

COPY . .

ARG VITE_API_URL
ENV VITE_API_URL=${VITE_API_URL}

RUN bun run build

CMD ["bun", "run", "serve", "--", "--host", "0.0.0.0", "--port", "4173"]
