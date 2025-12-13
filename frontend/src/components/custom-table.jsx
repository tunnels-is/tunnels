import { useState } from "react";
import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";


/**
 * @param {Object} param0 
 * @param {Array} param0.data 
 * @param {{id: string, header: string, accessorKey: string, cell: Function, header: Function}[]} param0.columns
 */
function CustomTable({ data, columns }) {
  const [searchValue, setSearchValue] = useState("");
  const [sorting, setSorting] = useState([]);

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    state: {
      globalFilter: searchValue,
      sorting,
    },
    onGlobalFilterChange: setSearchValue,
    onSortingChange: setSorting,
  });
  return (
    <div className="rounded-md border border-[#1a1f2d] overflow-hidden">
      <Table>
        <TableHeader className="bg-secondary">
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow
              key={headerGroup.id}
              className="border-b border-[#1a1f2d]"
            >
              {headerGroup.headers.map((header) => {
                return (
                  <TableHead key={header.id} className="text-white font-bold">
                    {header.isPlaceholder
                      ? null
                      : flexRender(
                        header.column.columnDef.header,
                        header.getContext()
                      )}
                  </TableHead>
                );
              })}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {
            table.getRowModel().rows?.length ? table.getRowModel().rows.map((row) => (
              <TableRow
                key={row.id}
                data-state={row.getIsSelected() && "selected"}
                className="border-b border-[#1a1f2d] hover:bg-muted/50 py-2"
                onClick={row.original.onClick}
              >
                {row.getVisibleCells().map((cell) => {
                  const metaClassName = cell.column.columnDef.meta?.className;
                  const className =
                    typeof metaClassName === "function"
                      ? metaClassName(row.original)
                      : metaClassName;

                  return (
                    <TableCell key={cell.id} className={className}>
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext()
                      )}
                    </TableCell>
                  );
                })}
              </TableRow>
            )) : (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-24 text-center">
                  No results.
                </TableCell>
              </TableRow>
            )
          }
        </TableBody>
      </Table>
    </div>

  );
}
export default CustomTable;