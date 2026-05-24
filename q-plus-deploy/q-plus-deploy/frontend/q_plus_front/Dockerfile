# v0.7.1

# Base node image
FROM node:18-alpine3.18 AS node

#RUN npm install -g node-gyp
#RUN apk --no-cache add curl

ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL

WORKDIR /app

COPY package.json ./
COPY package-lock.json ./

# Allow mounting of these files, which have no default
# values.
RUN touch .env
RUN npm config set fetch-retry-maxtimeout 600000
RUN npm config set fetch-retries 5
RUN npm config set fetch-retry-mintimeout 15000
RUN npm install --no-audit

COPY . .

# React client build
ENV NODE_OPTIONS="--max-old-space-size=2048"
RUN npm run build

# Optional: for client with nginx routing
FROM nginx:stable-alpine AS nginx-client
COPY --from=node /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
ENTRYPOINT ["nginx", "-g", "daemon off;"]
