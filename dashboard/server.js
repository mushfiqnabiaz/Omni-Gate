const http = require('http');
const https = require('https');
const { parse } = require('url');
const next = require('next');
const fs = require('fs');
const path = require('path');

const dev = process.env.NODE_ENV !== 'production';
const app = next({ dev });
const handle = app.getRequestHandler();

const httpsOptions = {
  key: fs.readFileSync(path.join(__dirname, 'key.pem')),
  cert: fs.readFileSync(path.join(__dirname, 'cert.pem')),
};

app.prepare().then(() => {
  // 1. The Secure Next.js Server
  https.createServer(httpsOptions, (req, res) => {
    const parsedUrl = parse(req.url, true);
    handle(req, res, parsedUrl);
  }).listen(443, (err) => {
    if (err) throw err;
    console.log('> Ready on https://omnigate.local');
  });

  // 2. The Automatic HTTP -> HTTPS Redirector
  http.createServer((req, res) => {
    res.writeHead(301, { "Location": "https://omnigate.local:8443" + req.url });
    res.end();
  }).listen(80, (err) => {
    if (err) throw err;
    console.log('> HTTP Redirector active on port 80');
  });
});
