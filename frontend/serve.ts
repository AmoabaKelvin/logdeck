import { serve } from "bun";
import { existsSync } from "fs";
import { join } from "path";

const distPath = join(__dirname, "dist");

serve({
  port: 3000,
  hostname: "0.0.0.0",
  async fetch(req) {
    const url = new URL(req.url);
    let filePath = join(
      distPath,
      url.pathname === "/" ? "index.html" : url.pathname
    );

    // Check if file exists
    if (!existsSync(filePath)) {
      // If not found, serve index.html for client-side routing
      filePath = join(distPath, "index.html");
    }

    try {
      const file = Bun.file(filePath);
      return new Response(file);
    } catch (error) {
      return new Response("Not Found", { status: 404 });
    }
  },
});
