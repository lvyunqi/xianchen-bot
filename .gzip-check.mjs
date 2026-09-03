// 验证 gzip 流完整性：模拟浏览器 fetch（Accept-Encoding: gzip + zlib 原生解压）
const res = await fetch("http://127.0.0.1:8089/api/dashboard", {
  headers: { "Accept-Encoding": "gzip" },
})
const enc = res.headers.get("content-encoding")
const buf = Buffer.from(await res.arrayBuffer())
console.log("status:", res.status, "encoding:", enc, "bytes:", buf.length)
if (enc === "gzip") {
  const zlib = await import("node:zlib")
  try {
    const text = zlib.gunzipSync(buf).toString("utf8")
    const json = JSON.parse(text)
    console.log("gzip OK, metrics:", JSON.stringify(json.data?.metrics))
  } catch (e) {
    console.log("GZIP BROKEN:", e.message, "head:", buf.slice(0, 40).toString("hex"))
  }
} else {
  const text = buf.toString("utf8")
  console.log("plain:", text.slice(0, 200))
}
