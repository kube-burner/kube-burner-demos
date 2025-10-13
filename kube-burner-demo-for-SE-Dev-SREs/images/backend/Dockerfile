FROM node:20-alpine AS base
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install --only=production
COPY server.js ./
EXPOSE 8080
CMD ["npm", "start"]
