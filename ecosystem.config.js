module.exports = {
  apps: [
    {
      name: "omnigate-gateway",
      script: "./bin/gateway",
      cwd: "./gateway",
      watch: false,
      env: {
        PORT: 8050,
      }
    },
    {
      name: "omnigate-dashboard",
      script: "node",
      args: "server.js",
      cwd: "./dashboard",
      watch: false,
      env: {
        NODE_ENV: "production",
      }
    }
  ]
};
