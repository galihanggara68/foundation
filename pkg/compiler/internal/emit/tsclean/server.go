package tsclean

// emitTSServer renders the backend entry point (src/main/server.ts): it constructs the real
// express application and pg connection pool, bridges them to the controller ports, and starts
// listening. This is the only emitted file that imports express and pg directly.
func emitTSServer() string {
	return `import express, { type Express, type Request, type Response } from "express";
import { Pool } from "pg";
import {
  type ExpressRequest,
  type ExpressResponse,
  type ExpressRouterLike,
} from "../controllers/express_router.ts";
import { createPostgresContainer, createTestContainer } from "./container.ts";

// adaptExpress bridges the real express application to the ExpressRouterLike port the
// controllers depend on, keeping the HTTP framework an exchangeable composition detail.
function adaptExpress(app: Express): ExpressRouterLike {
  const adaptHandler =
    (handler: (req: ExpressRequest, res: ExpressResponse) => Promise<void>) =>
    (req: Request, res: Response): void => {
      void handler(req as unknown as ExpressRequest, res as unknown as ExpressResponse);
    };
  return {
    get: (path, handler) => app.get(path, adaptHandler(handler)),
    post: (path, handler) => app.post(path, adaptHandler(handler)),
    put: (path, handler) => app.put(path, adaptHandler(handler)),
    delete: (path, handler) => app.delete(path, adaptHandler(handler)),
  };
}

const app: Express = express();
app.use(express.json());

let controller: { registerRoutes(router: ExpressRouterLike): void };
if (process.env.DATABASE_URL) {
  const db = new Pool({
    connectionString: process.env.DATABASE_URL,
  });
  controller = createPostgresContainer(db).controller;
} else {
  controller = createTestContainer().controller;
}
controller.registerRoutes(adaptExpress(app));

const port = Number(process.env.PORT ?? "8080");
const server = app.listen(port);

process.on("SIGTERM", () => {
  server.close(() => {
    process.exit(0);
  });
});
`
}
