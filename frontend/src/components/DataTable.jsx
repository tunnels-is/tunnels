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
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useState, useEffect } from "react";
import { ArrowLeft, ArrowRight } from "lucide-react";

export function DataTable({
  columns,
  data,
  pagination = true,
  sorting = true,
  filtering = true,
  globalFilter,
  setGlobalFilter,
  pageCount,
  onPaginationChange,
  manualPagination = false,
  state,
  showSearch = true,
}) {
  const [localSorting, setLocalSorting] = useState([]);
  const [localGlobalFilter, setLocalGlobalFilter] = useState("");

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: pagination ? getPaginationRowModel() : undefined,
    getSortedRowModel: sorting ? getSortedRowModel() : undefined,
    getFilteredRowModel: filtering ? getFilteredRowModel() : undefined,
    onSortingChange: setLocalSorting,
    onGlobalFilterChange: setGlobalFilter || setLocalGlobalFilter,
    manualPagination: manualPagination,
    pageCount: pageCount,
    state: {
      sorting: localSorting,
      globalFilter: globalFilter ?? localGlobalFilter,
      ...state,
    },
    onPaginationChange: onPaginationChange,
  });

  return (
    <div className="w-full">
      {showSearch && (
        <div className="flex items-center py-4">
          <Input
            placeholder="Search..."
            value={globalFilter ?? localGlobalFilter ?? ""}
            onChange={(event) =>
              (setGlobalFilter || setLocalGlobalFilter)(event.target.value)
            }
            className="max-w-sm"
          />
        </div>
      )}
      <div className="rounded-md border border-[#1a1f2d] overflow-hidden">
        <Table>
          <TableHeader className="bg-[#0B0E14]">
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
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
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
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className="h-24 text-center"
                >
                  No results.
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      {pagination && !manualPagination && (
        <div className="flex items-center justify-end space-x-2 py-4">
          <Button
            variant="outline"
            size="sm"
            onClick={() => table.previousPage()}
            disabled={!table.getCanPreviousPage()}
          >
            <ArrowLeft className="w-4 h-4 mr-2" />
            Previous
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => table.nextPage()}
            disabled={!table.getCanNextPage()}
          >
            Next
            <ArrowRight className="w-4 h-4 ml-2" />
          </Button>
        </div>
      )}
    </div>
  );
}
