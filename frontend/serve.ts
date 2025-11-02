import { serve } from "bun";

import indexHtml from "./index.html";

serve({
  port: 3000,
  hostname: "0.0.0.0",
  routes: {
    "/": indexHtml,
  },
});
