import { ContainerTable } from "./container-table";
import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import {
  api,
  base,
  labels,
  useData,
  type Container,
  type Inventory,
  type Deployments,
  type Settings,
  type Job,
} from "./data";
import { Badge } from "./components/ui/badge";

export function Action({
  kind,
  id,
  scope,
  allowed,
  reason,
  actions,
  service,
}: {
  kind: string;
  id: string;
  scope: string;
  allowed: boolean;
  reason: string;
  actions: string[];
  service?: string;
}) {
  const client = useQueryClient();
  const op = useMutation({
    mutationFn: (action: string) =>
      api(
        "/actions/" +
          kind +
          "/" +
          encodeURIComponent(id) +
          "/" +
          action +
          (service ? "?service=" + encodeURIComponent(service) : ""),
        { method: "POST", headers: { "X-Doco-UI": "1" } },
      ),
    onSettled: () => client.invalidateQueries({ queryKey: ["local"] }),
  });
  return (
    <div className="actions">
      {actions.map((action) => (
        <button
          key={action}
          disabled={!allowed || op.isPending}
          onClick={() => {
            const note =
              action === "stop"
                ? " Die Git-Konfiguration bleibt bestehen."
                : "";
            if (window.confirm(scope + " – " + labels[action] + "?" + note))
              op.mutate(action);
          }}
        >
          {labels[action]}
        </button>
      ))}
      <small role="status">
        {!allowed
          ? reason
          : op.isPending
            ? "Wird ausgeführt …"
            : (op.error?.message ??
              (op.isSuccess
                ? kind === "sources"
                  ? "Auftrag angenommen"
                  : "Aktion abgeschlossen"
                : ""))}
      </small>
    </div>
  );
}
function Note({ text }: { text?: string }) {
  return text ? (
    <p role="status" className="note">
      {text}
    </p>
  ) : null;
}
function ContainerLink({ c }: { c: Container }) {
  return (
    <Link to="/containers/$id" params={{ id: c.ID }}>
      {c.Name}
    </Link>
  );
}
function State({ value }: { value: string }) {
  return <Badge variant="outline">{labels[value] ?? value}</Badge>;
}
function SourcePanel({
  settings,
  available,
}: {
  settings: Settings;
  available: boolean;
}) {
  const sources = settings.sources ?? [];
  const [id, setID] = useState("");
  const source = sources.find((s) => s.ID === id) ?? sources[0];
  const jobs = useData<Record<string, Job>>("/jobs", 2500);
  const job = source && jobs.data?.[source.ID];
  const busy = job && ["accepted", "running"].includes(job.Run.Status);
  return (
    <section>
      <h2>GitOps-Abgleich</h2>
      {source ? (
        <>
          <label>
            Quelle{" "}
            <select value={source.ID} onChange={(e) => setID(e.target.value)}>
              {sources.map((s) => (
                <option key={s.ID} value={s.ID}>
                  {s.Name}
                </option>
              ))}
            </select>
          </label>
          <p className="mono">
            {source.URL} · {source.Reference || "Standard-Referenz"} ·{" "}
            {source.Target || "Standard-Konfiguration"}
          </p>
          <p>Umfang: {source.Scope}</p>
          <Action
            key={source.ID}
            kind="sources"
            id={source.ID}
            scope={source.Name + " · " + source.Scope}
            actions={["sync"]}
            allowed={
              settings.poll &&
              available &&
              !busy &&
              !!jobs.data &&
              !jobs.isError
            }
            reason={
              busy
                ? "Abgleich läuft"
                : !settings.poll
                  ? "Abgleich deaktiviert"
                  : "Laufstatus nicht verfügbar"
            }
          />
          {job && (
            <p aria-live="polite">
              <State value={job.Run.Status} /> <code>{job.Run.job_id}</code>{" "}
              {job.Error}
            </p>
          )}
          <Note text={jobs.error?.message} />
        </>
      ) : (
        <p>Keine GitOps-Quellen konfiguriert.</p>
      )}
    </section>
  );
}
export function Dashboard() {
  const inv = useData<Inventory>("/inventory"),
    cd = useData<Deployments>("/deployments"),
    cfg = useData<Settings>("/settings");
  const containers = inv.data?.Containers ?? [];
  return (
    <>
      <h1>Übersicht</h1>
      <div className="grid gap-4 md:grid-cols-2">
        <section>
          <h2>Docker Engine</h2>
          <Note text={inv.error?.message} />
          {inv.data ? (
            <>
              <p>
                <strong>{containers.length}</strong> Container ·{" "}
                {containers.filter((c) => c.State === "running").length} laufen
              </p>
              <p>
                {new Set(containers.map((c) => c.Project).filter(Boolean)).size}{" "}
                Compose-Projekte · {inv.data.Networks.length} Netzwerke
              </p>
            </>
          ) : (
            !inv.error && "Verbindung wird geprüft …"
          )}
        </section>
        <section>
          <h2>doco-cd</h2>
          <Note text={cd.error?.message || cd.data?.ProjectsError} />
          {cd.data && !cd.data.ProjectsError ? (
            <>
              {cd.data.Projects?.length
                ? cd.data.Projects.map((p) => (
                    <p key={p.Name}>
                      {p.Name} · {p.Status}
                    </p>
                  ))
                : "Verbunden · keine Compose-Projekte"}
            </>
          ) : (
            !cd.error && !cd.data && "Verbindung wird geprüft …"
          )}
        </section>
      </div>
      {cfg.data && (
        <SourcePanel
          settings={cfg.data}
          available={!!cd.data && !cd.data.RunsError && !cd.error}
        />
      )}
      <Note text={cfg.error?.message} />
      <section>
        <h2>Letzte Deployment-Läufe</h2>
        <Note text={cd.error?.message || cd.data?.RunsError} />
        {cd.data &&
          !cd.data.RunsError &&
          ((cd.data.Runs ?? []).length
            ? cd.data.Runs!.map((run) => (
                <article key={run.job_id}>
                  <State value={run.Status} /> {run.Repository || run.Trigger} ·{" "}
                  {new Date(run.created_at).toLocaleString("de-DE")}
                  <p className="mono">{run.job_id}</p>
                  {run.Deployments?.map((d) => d.Context + "/" + d.Stack).join(
                    ", ",
                  )}
                </article>
              ))
            : "Keine Läufe vorhanden.")}
      </section>
    </>
  );
}
export function Resources({
  page,
  search = "",
  status = "",
  change,
}: {
  page: string;
  search?: string;
  status?: string;
  change?: (q: string, state: string) => void;
}) {
  const inv = useData<Inventory>("/inventory"),
    cd = useData<Deployments>("/deployments"),
    cfg = useData<Settings>("/settings");
  const all = inv.data?.Containers ?? [],
    projects = [
      ...new Set([
        ...all.map((c) => c.Project).filter(Boolean),
        ...(cd.data?.Projects ?? []).map((p) => p.Name),
      ]),
    ].sort();
  const filtered = all.filter(
    (c) =>
      (!status ||
        c.State === status ||
        (status === "unhealthy" && c.Status.includes("(unhealthy)"))) &&
      [c.Name, c.Image, c.Project, c.Service]
        .join(" ")
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  return (
    <>
      <h1>{page}</h1>
      <Note text={inv.error?.message} />
      {inv.isPending && <p>Docker-Inventar wird geladen …</p>}
      {page === "Container" && (
        <>
          <div className="actions">
            <label>
              Suche{" "}
              <input
                type="search"
                value={search}
                onChange={(e) => change?.(e.target.value, status)}
              />
            </label>
            <label>
              Status{" "}
              <select
                value={status}
                onChange={(e) => change?.(search, e.target.value)}
              >
                <option value="">Alle</option>
                {[
                  ...new Set([
                    ...all.map((c) => c.State),
                    "unhealthy",
                    ...(status ? [status] : []),
                  ]),
                ].map((s) => (
                  <option key={s} value={s}>
                    {labels[s] ?? s}
                  </option>
                ))}
              </select>
            </label>
          </div>
          {Array.from(new Set(filtered.map((c) => c.Project)))
            .sort()
            .map((p) => (
              <section key={p}>
                <h2>{p || "Ohne Compose-Projekt"}</h2>
                <ContainerTable
                  items={filtered.filter((c) => c.Project === p)}
                />
              </section>
            ))}
          {inv.data && !filtered.length && <p>Keine passenden Container.</p>}
        </>
      )}
      {page === "Netzwerke" &&
        inv.data?.Networks.map((n) => (
          <section key={n.ID}>
            <h2>{n.Name}</h2>
            <p>
              {n.Driver} ·{" "}
              {n.IPAM.Config?.map((c) => c.Subnet)
                .filter(Boolean)
                .join(", ") || "Kein Subnetz gemeldet"}
            </p>
            {all
              .filter((c) => c.Networks?.[n.Name]?.EndpointID)
              .map((c) => (
                <article key={c.ID}>
                  <ContainerLink c={c} /> ·{" "}
                  {[
                    c.Networks![n.Name].IPAddress,
                    c.Networks![n.Name].GlobalIPv6Address,
                  ]
                    .filter(Boolean)
                    .join(" · ") || "Keine IP-Adresse"}
                </article>
              ))}
          </section>
        ))}
      {page === "Compose-Projekte" && (
        <>
          <Note text={cd.error?.message || cd.data?.ProjectsError} />
          {projects.map((p) => {
            const members = all.filter((c) => c.Project === p),
              managed = !!members.length && members.every((c) => c.Managed),
              project = cd.data?.Projects?.find((v) => v.Name === p);
            const allowed = !!(
              cfg.data?.control &&
              managed &&
              project &&
              !inv.isError &&
              !cd.isError &&
              !cd.data?.ProjectsError
            );
            const reason = !managed
              ? "Keine vollständige doco-cd-Zuordnung"
              : !cfg.data?.control
                ? "Projektsteuerung deaktiviert"
                : "doco-cd-Projekt nicht verfügbar";
            return (
              <section key={p}>
                <h2>{p}</h2>
                <p>
                  {project?.Status} · {members.filter((c) => c.Managed).length}/
                  {members.length} Container von doco-cd verwaltet
                </p>
                <Action
                  kind="projects"
                  id={p}
                  scope={"Gesamtes Projekt " + p}
                  actions={["start", "stop", "restart", "recreate"]}
                  allowed={allowed}
                  reason={reason}
                />
                {Array.from(new Set(members.map((c) => c.Service))).map((s) => (
                  <article key={s}>
                    <h3>{s || "Ohne Service"}</h3>
                    {members
                      .filter((c) => c.Service === s)
                      .map((c) => (
                        <p key={c.ID}>
                          <ContainerLink c={c} /> · {c.Status}
                        </p>
                      ))}
                    {s && allowed && (
                      <Action
                        kind="projects"
                        id={p}
                        scope={p + " / " + s}
                        service={s}
                        actions={["recreate"]}
                        allowed
                        reason=""
                      />
                    )}
                  </article>
                ))}
                {(cd.data?.Runs ?? [])
                  .filter((r) =>
                    r.Deployments?.some(
                      (d) => d.Context === "default" && d.Stack === p,
                    ),
                  )
                  .map((r) => (
                    <p key={r.job_id}>
                      Deployment {r.job_id} · <State value={r.Status} />
                    </p>
                  ))}
              </section>
            );
          })}
          {!projects.length && inv.data && (
            <p>Keine Compose-Projekte gefunden.</p>
          )}
        </>
      )}
    </>
  );
}

function Metrics({ id }: { id: string }) {
  type Point = { Time: string; CPU: number | null; Bytes: number };
  const [series, setSeries] = useState<Point[]>([]),
    [error, setError] = useState("");
  useEffect(() => {
    const stream = new EventSource(base + "/containers/" + id + "/metrics");
    stream.onmessage = (e) => {
      setSeries(JSON.parse(e.data));
      setError("");
    };
    stream.onerror = () => setError("Messverbindung unterbrochen");
    stream.addEventListener("unavailable", (e) => setError(JSON.parse(e.data)));
    return () => stream.close();
  }, [id]);
  const format = (p: Point) =>
    (p.CPU === null ? "–" : p.CPU.toFixed(1) + " %") +
    " CPU · " +
    (p.Bytes / 1048576).toFixed(1) +
    " MiB RAM";
  return (
    <section>
      <h2>Container-Auslastung</h2>
      <Note text={error} />
      {series.length ? (
        <>
          <p>{format(series[series.length - 1])}</p>
          <details>
            <summary>Messverlauf · letzte fünf Minuten</summary>
            {series
              .slice()
              .reverse()
              .map((p) => (
                <p className="mono" key={p.Time}>
                  {new Date(p.Time).toLocaleTimeString("de-DE")} · {format(p)}
                </p>
              ))}
          </details>
        </>
      ) : (
        !error && "Messung läuft …"
      )}
    </section>
  );
}
function Logs({ id }: { id: string }) {
  const [mode, setMode] = useState<null | boolean>(null),
    [output, setOutput] = useState(""),
    [state, setState] = useState("");
  useEffect(() => {
    if (mode === null) return;
    const ac = new AbortController();
    setOutput("");
    setState("Lädt …");
    async function read() {
      try {
        const res = await fetch(
          base + "/containers/" + id + "/logs?follow=" + (mode ? "1" : "0"),
          { signal: ac.signal },
        );
        if (!res.ok || !res.body) throw new Error(await res.text());
        const reader = res.body
          .pipeThrough(new TextDecoderStream())
          .getReader();
        setState(mode ? "Live" : "Lädt …");
        while (true) {
          const chunk = await reader.read();
          if (chunk.done) break;
          setOutput((prev) => (prev + chunk.value).slice(-262144));
        }
        setState("Ausgabe abgeschlossen");
      } catch (e) {
        if (!ac.signal.aborted) setState(String(e));
      }
    }
    void read();
    return () => ac.abort();
  }, [id, mode]);
  return (
    <section>
      <h2>Container-Logs</h2>
      <div className="actions">
        <button onClick={() => setMode(false)} disabled={mode === false}>
          Letzte 200 Zeilen
        </button>
        <button onClick={() => setMode(true)} disabled={mode === true}>
          Live verfolgen
        </button>
        {mode !== null && (
          <button onClick={() => setMode(null)}>Schließen</button>
        )}
      </div>
      {mode !== null && (
        <>
          <small>{state}</small>
          <pre>{output}</pre>
        </>
      )}
    </section>
  );
}
export function Detail({ id }: { id: string }) {
  const inv = useData<Inventory>("/inventory"),
    settings = useData<Settings>("/settings"),
    health = useData<{ Status: string } | null>(
      "/containers/" + id + "/health",
    );
  const c = inv.data?.Containers.find((c) => c.ID === id);
  if (!c)
    return (
      <p>
        {inv.error?.message ??
          (inv.isPending ? "Lädt …" : "Container nicht gefunden.")}
      </p>
    );
  const actions =
    c.State === "running"
      ? ["stop", "restart"]
      : ["created", "exited"].includes(c.State)
        ? ["start"]
        : [];
  return (
    <>
      <Link to="/containers">← Container</Link>
      <h1>{c.Name}</h1>
      <section>
        <h2>Containerdetails</h2>
        <State value={c.State} /> {c.Status}
        <dl>
          {Object.entries({
            Image: c.Image,
            Projekt: c.Project,
            Service: c.Service,
            Verwaltung: c.Managed ? "doco-cd" : "Eigenständig",
            Healthcheck:
              health.data?.Status ??
              (health.isPending
                ? "Lädt …"
                : (health.error?.message ?? "Keiner")),
            Ports: c.Ports?.map(
              (p) =>
                (p.PublicPort
                  ? (p.IP || "*") + ":" + p.PublicPort + " → "
                  : "") +
                p.PrivatePort +
                "/" +
                p.Type,
            ).join(", "),
            Mounts: c.Mounts?.map(
              (m) =>
                m.Source +
                " → " +
                m.Destination +
                " (" +
                (m.RW ? "rw" : "ro") +
                ")",
            ).join(", "),
            Netzwerke: Object.keys(c.Networks ?? {}).join(", "),
          }).map(([k, v]) => (
            <div key={k}>
              <dt>{k}</dt>
              <dd>{v || "–"}</dd>
            </div>
          ))}
        </dl>
        <Action
          kind="containers"
          id={id}
          scope={"Container " + c.Name}
          allowed={!!settings.data?.write && !!actions.length && !inv.isError}
          reason={
            actions.length
              ? "Schreibzugriff nicht verfügbar"
              : "Keine Aktion für diesen Zustand"
          }
          actions={actions}
        />
      </section>
      {c.State === "running" && <Metrics id={id} />}
      <Logs id={id} />
    </>
  );
}
