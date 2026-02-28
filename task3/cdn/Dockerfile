FROM nginx:alpine
RUN mkdir -p /var/cache/nginx/cdn && chown nginx:nginx /var/cache/nginx/cdn
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 8080
