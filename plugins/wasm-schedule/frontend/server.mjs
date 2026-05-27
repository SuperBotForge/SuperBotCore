import { createReadStream, existsSync, statSync } from "node:fs";
import { createServer } from "node:http";
import { extname, join, normalize, resolve } from "node:path";

const host = process.env.SCHEDULE_FRONTEND_HOST || "127.0.0.1";
const port = Number(process.env.SCHEDULE_FRONTEND_PORT || 5173);
const coreURL = (process.env.SCHEDULE_CORE_URL || "http://127.0.0.1:4000").replace(/\/+$/, "");
const root = resolve(import.meta.dirname);

const contentTypes = {
  ".css": "text/css; charset=utf-8",
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".md": "text/markdown; charset=utf-8",
};

createServer((req, res) => {
  const url = new URL(req.url || "/", `http://${host}:${port}`);

  if (url.pathname === "/oauth/login") {
    res.writeHead(302, { Location: coreURL + url.pathname + url.search });
    res.end();
    return;
  }

  if (url.pathname === "/favicon.ico") {
    res.writeHead(204);
    res.end();
    return;
  }

  const pathname = url.pathname === "/" ? "/index.html" : url.pathname;
  const filePath = resolve(root, "." + normalize(pathname));
  if (!filePath.startsWith(root + "/") || !existsSync(filePath) || !statSync(filePath).isFile()) {
    res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" });
    res.end("File not found");
    return;
  }

  res.writeHead(200, { "Content-Type": contentTypes[extname(filePath)] || "application/octet-stream" });
  createReadStream(filePath).pipe(res);
}).listen(port, host, () => {
  console.log(`Schedule frontend: http://${host}:${port}/`);
  console.log(`OAuth callback bridge: http://${host}:${port}/oauth/login -> ${coreURL}/oauth/login`);
});
