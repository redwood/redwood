import dgram from "dgram";
import express from "express";
import path from "path";
import fs from "fs";
import mediaServer from "./lib/media-server";
import * as _ from 'lodash'

import * as Braid from '/home/bryn/projects/forest/braidjs/braid-src'

mediaServer.run();

const app = express();
const PORT = 8081;
const HOST = "127.0.0.1";
const streamPath = path.join(__dirname, "learn-with-coffee-livestream.mp4");

// Middleware
app.use("/static", express.static("public"));

const updServer = dgram.createSocket("udp4");

updServer.on("listening", () => {
  const { address, port } = updServer.address();
  console.log(address, port);
  console.log(`UPD server listening on ${address}:${port}`);
});

const inStream = fs.createWriteStream(streamPath);
updServer.on("message", (message, remote) => {
  // console.log(message)
  inStream.write(message);
});

app.get("/coffee-live-stream", (req, res) => {
  res.sendFile(path.join(__dirname, "index.html"));
});

app.get("/live-stream", (req, res) => {
  const { range } = req.headers;
  const stat = fs.statSync("learn-with-coffee-livestream.mp4");
  const fileSize = stat.size;
  console.log(fileSize);
  if (range) {
    console.log(`RANGE HIT ${range}`);
    const parts = range.replace(/bytes=/, "").split("-");
    const start = parseInt(parts[0], 10);
    const end = parts[1] ? parseInt(parts[1], 10) : fileSize - 1;
    const chunksize = end - start + 1;
    const file = fs.createReadStream(streamPath, { start, end });
    const head = {
      "Content-Range": `bytes ${start}-${end}/${fileSize}`,
      "Accept-Ranges": "bytes",
      "Content-Length": chunksize,
      "Content-Type": "video/mp4",
    };
    res.writeHead(206, head);
    file.pipe(res);
  } else {
    console.log(range);
    const head = {
      "Content-Length": fileSize,
      "Content-Type": "video/mp4",
    };

    res.writeHead(200, head);
    fs.createReadStream(streamPath).pipe(res);
  }
});

updServer.bind(PORT, HOST);

app.listen(1337, () => {
  console.log("App running on port 1337");
});
