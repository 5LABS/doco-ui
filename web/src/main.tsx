import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import {
  QueryClient,
  QueryClientProvider,
  useQuery,
} from "@tanstack/react-query";
import {
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  Outlet,
  RouterProvider,
} from "@tanstack/react-router";
import { Dashboard, Detail, Resources } from "./views";
import icon from "../../assets/icon.svg";
import "./styles.css";

const resources = [
  { path: "/containers", name: "Container", source: "Docker Engine" },
  {
    path: "/projects",
    name: "Compose-Projekte",
    source: "Docker Engine und doco-cd",
  },
  { path: "/networks", name: "Netzwerke", source: "Docker Engine" },
] as const;

function Layout() {
  const target = useQuery({
    queryKey: ["targets", "local"],
    queryFn: async ({ signal }): Promise<{ id: string; name: string }> => {
      const response = await fetch("/api/targets/local", { signal });
      if (!response.ok) throw new Error("Ziel konnte nicht geladen werden.");
      return response.json();
    },
  });
  return (
    <>
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-5 py-5">
          <Link
            to="/"
            className="flex items-center gap-3 text-xl font-semibold tracking-tight"
          >
            <img src={icon} width="36" height="36" alt="" />
            doco-ui
          </Link>
          <span aria-live="polite" className="text-sm text-slate-600">
            {target.data?.name ??
              (target.isError
                ? "Ziel nicht erreichbar"
                : "Ziel wird geladen …")}
          </span>
        </div>
        <nav
          aria-label="Hauptnavigation"
          className="mx-auto flex max-w-5xl flex-wrap gap-x-5 px-5"
        >
          {[{ path: "/", name: "Übersicht" }, ...resources].map((item) => (
            <Link
              key={item.path}
              to={item.path}
              activeOptions={{ exact: true }}
              className="border-b-2 border-transparent py-3 text-sm text-slate-600 hover:text-sky-800"
              activeProps={{
                className: "border-sky-600 font-semibold text-sky-800",
              }}
            >
              {item.name}
            </Link>
          ))}
        </nav>
      </header>
      <main className="mx-auto max-w-5xl px-5 py-10">
        <Outlet />
      </main>
    </>
  );
}

const heading = "mb-6 text-3xl font-semibold tracking-tight";
const root = createRootRoute({
  component: Layout,
  notFoundComponent: () => (
    <>
      <h1 className={heading}>Seite nicht gefunden</h1>
      <Link className="text-sky-800 underline" to="/">
        Zur Übersicht
      </Link>
    </>
  ),
});
const overview = createRoute({
  getParentRoute: () => root,
  path: "/",
  component: Dashboard,
});
const containers = createRoute({
  getParentRoute: () => root,
  path: "/containers",
  validateSearch: (
    s: Record<string, unknown>,
  ): { q?: string; state?: string } => ({
    q: typeof s.q === "string" ? s.q : undefined,
    state: typeof s.state === "string" ? s.state : undefined,
  }),
  component: ContainerPage,
});
function ContainerPage() {
  const { q, state } = containers.useSearch(),
    navigate = containers.useNavigate();
  return (
    <Resources
      page="Container"
      search={q}
      status={state}
      change={(q, state) =>
        void navigate({
          search: { q: q || undefined, state: state || undefined },
          replace: true,
        })
      }
    />
  );
}
const detail = createRoute({
  getParentRoute: () => root,
  path: "/containers/$id",
  component: () => {
    const { id } = detail.useParams();
    return <Detail key={id} id={id} />;
  },
});
const projects = createRoute({
  getParentRoute: () => root,
  path: "/projects",
  component: () => <Resources page="Compose-Projekte" />,
});
const networks = createRoute({
  getParentRoute: () => root,
  path: "/networks",
  component: () => <Resources page="Netzwerke" />,
});
const router = createRouter({
  routeTree: root.addChildren([
    overview,
    containers,
    detail,
    projects,
    networks,
  ]),
});
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
const client = new QueryClient();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
