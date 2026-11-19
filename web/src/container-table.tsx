import { Link } from "@tanstack/react-router";
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import { labels, type Container } from "./data";

const columns: ColumnDef<Container>[] = [
  {
    accessorKey: "Name",
    header: "Container",
    cell: ({ row }) => (
      <Link to="/containers/$id" params={{ id: row.original.ID }}>
        {row.original.Name}
      </Link>
    ),
  },
  {
    accessorKey: "State",
    header: "Zustand",
    cell: ({ row }) => (
      <>
        {labels[row.original.State] ?? row.original.State}
        <small className="block">{row.original.Status}</small>
      </>
    ),
  },
  { accessorKey: "Image", header: "Image" },
];
export function ContainerTable({ items }: { items: Container[] }) {
  const table = useReactTable({
    data: items,
    columns,
    getRowId: (c) => c.ID,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    initialState: { sorting: [{ id: "Name", desc: false }] },
    autoResetPageIndex: false,
  });
  const image = table.getColumn("Image")!;
  return (
    <>
      <label className="mb-2">
        <input
          type="checkbox"
          checked={image.getIsVisible()}
          onChange={image.getToggleVisibilityHandler()}
        />{" "}
        Image anzeigen
      </label>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead>
            {table.getHeaderGroups().map((group) => (
              <tr key={group.id}>
                {group.headers.map((h) => {
                  const order = h.column.getIsSorted();
                  return (
                    <th
                      scope="col"
                      key={h.id}
                      aria-sort={
                        order === "asc"
                          ? "ascending"
                          : order === "desc"
                            ? "descending"
                            : "none"
                      }
                    >
                      <button onClick={h.column.getToggleSortingHandler()}>
                        {flexRender(h.column.columnDef.header, h.getContext())}{" "}
                        {order === "asc" ? "↑" : order === "desc" ? "↓" : "↕"}
                      </button>
                    </th>
                  );
                })}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map((row) => (
              <tr key={row.id} className="border-t border-slate-200">
                {row.getVisibleCells().map((cell) => (
                  <td className="py-3 pr-3" key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
