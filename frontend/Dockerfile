FROM node:18-alpine as build

WORKDIR /app

# Install dependencies
COPY package.json .
RUN npm install

# Copy source and build
COPY . .
RUN npm run build

# Production stage with nginx
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 3000
CMD ["nginx", "-g", "daemon off;"]
