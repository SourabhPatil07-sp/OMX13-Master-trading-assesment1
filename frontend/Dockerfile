FROM node:20-alpine

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm install

# Copy project files
COPY . .

# Expose Vite's default port
EXPOSE 5173

# Run the dev server and bind to all network interfaces so it's accessible outside the container
CMD ["npm", "run", "dev", "--", "--host"]
