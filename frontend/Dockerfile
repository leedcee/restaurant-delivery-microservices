FROM node:24-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.31.7-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
RUN chown -R nginx:nginx /var/cache/nginx /var/run /usr/share/nginx/html
USER nginx
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT []
CMD ["nginx", "-g", "daemon off;"]
