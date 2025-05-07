import React, { useState } from "react";
import GLOBAL_STATE from "../../state";
import STORE from "../../store";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  ChevronLeft,
  ChevronRight,
  Search,
  ArrowRight,
  Filter,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";

const NewTable = (props) => {
  const [filter, setFilter] = useState("");
  const state = GLOBAL_STATE(props.tableID);

  const arrayFromTotalPages = (total) => {
    if (total > 0) {
      return [...Array(total).keys()];
    } else {
      return [];
    }
  };

  const setPageWrap = (page, totalPages) => {
    if (page === totalPages) {
      page = totalPages - 1;
    } else if (page < 0) {
      page = 0;
    }

    state.setPage(props.tableID, page);
  };

  // Improved filter function with case-insensitive search
  let finalRows = [];
  if (filter !== "") {
    props?.rows?.forEach((r) => {
      let show = false;
      const lowercaseFilter = filter.toLowerCase();

      r.items.forEach((item) => {
        if (String(item.value).toLowerCase().includes(lowercaseFilter)) {
          show = true;
        }
      });

      if (show === true) {
        finalRows.push(r);
      }
    });
  } else {
    finalRows = props.rows || [];
  }

  let pg = STORE.Cache.GetObject("table_" + props.tableID);
  let showNP = true;
  let showPP = true;
  let originalSize = 0;
  if (pg) {
    originalSize = pg.TableSize;
    if (pg.TableSize === 0) {
      pg.TableSize = finalRows.length;
    }
    pg.TotalPages = Math.ceil(finalRows.length / pg.TableSize);
    if (pg.TotalPages === 0) {
      pg.TotalPages = 1;
    }
    if (pg.CurrentPage < 0) {
      pg.CurrentPage = 0;
    } else if (pg.CurrentPage > pg.TotalPages - 1) {
      pg.CurrentPage = pg.TotalPages - 1;
    }

    pg.NextPage = pg.CurrentPage + 1;
    if (pg.NextPage > pg.TotalPages) {
      showNP = false;
    }

    pg.PrevPage = pg.CurrentPage - 1;
    if (pg.PrevPage < 0) {
      showPP = false;
    }
  } else {
    pg = {
      TableSize: 20,
      CurrentPage: 0,
      NextPage: 1,
      PrevPage: -1,
    };
    pg.TotalPages = Math.ceil(finalRows.length / pg.TableSize);
    STORE.Cache.SetObject("table_" + props.tableID, pg);
  }

  let indexes = [];
  let x = pg.CurrentPage * pg.TableSize;
  let fin = x + pg.TableSize - 1;
  for (var i = x; i < fin; i++) {
    if (i < finalRows.length) {
      indexes.push(i);
    } else {
      break;
    }
  }

  // Calculate showing items text
  const calculateItemsShowing = () => {
    if (finalRows.length === 0) return "0 items";

    const start = pg.CurrentPage * pg.TableSize + 1;
    const end = Math.min((pg.CurrentPage + 1) * pg.TableSize, finalRows.length);
    return `${start}-${end} of ${finalRows.length}`;
  };

  // Render a card-style row for private VPN servers
  const renderServers = (row, index) => {
    const statusItem = row.items.find(
      (item) => item.s_type === "connect-disconnect",
    );
    const assignItem = row.items.find((item) => item.type === "select");

    const tag = row.items[0];
    const ip = row.items[2];
    const country = row.items[3];
    const port = row.items[4];
    const dataport = row.items[5];

    return (
      <div
        key={`private-card-${index}`}
        className="flex items-center p-4 bg-[#0a0a0a] border border-[#222] rounded-lg hover:border-[#333] transition-all shadow-md hover:shadow-lg"
      >
        <div className="flex-1 min-w-0">
          {tag?.value && (
            <h3
              onClick={() => tag.click && tag.click()}
              className="text-white text-base font-medium cursor-pointer hover:text-blue-400 transition-colors"
            >
              {tag.value}
            </h3>
          )}

          <div className="flex flex-wrap gap-2 items-center mt-1 text-xs">
            {ip?.value && (
              <div className="px-2 py-0.5 bg-emerald-500/10 text-emerald-400 rounded font-mono">
                {ip.value}
              </div>
            )}
            {port?.value && (
              <div className="px-2 py-0.5 bg-purple-500/10 text-purple-400 rounded">
                Port: {port.value}
              </div>
            )}
            {dataport?.value && (
              <div className="px-2 py-0.5 bg-purple-500/10 text-purple-400 rounded">
                DataPort: {dataport.value}
              </div>
            )}
          </div>
        </div>

        <div className="flex items-center gap-3">
          {assignItem && assignItem.value}

          {statusItem && (
            <Badge
              onClick={() => statusItem.click && statusItem.click()}
              className={`
								font-medium text-xs py-1 px-2.5 rounded-md border cursor-pointer
								${
                  statusItem.s_state === "connect"
                    ? "bg-green-500/10 text-green-400 border-green-500/20 hover:bg-green-500/20"
                    : "bg-red-500/10 text-red-400 border-red-500/20 hover:bg-red-500/20"
                }
							`}
            >
              {statusItem.value}
            </Badge>
          )}
        </div>
      </div>
    );
  };

  return (
    <div
      className={`${!props.design && "max-w-[900px]"} space-y-3 transition-all duration-300 ${props.background ? "bg-[#000000] p-6 rounded-md shadow-lg " : ""} ${props.className}`}
    >
      <div className="flex items-center justify-between gap-2">
        {props?.title && (
          <div className="flex items-center">
            <h2 className="text-lg font-semibold tracking-tight text-white relative pl-2 before:content-[''] before:absolute before:left-0 before:top-0 before:bottom-0 before:w-1 before:bg-blue-500 before:rounded-full">
              {props.title}
            </h2>
          </div>
        )}

        <div className="flex items-center gap-2">
          {props?.button && (
            <Button
              variant="outline"
              size="sm"
              className="h-8 px-3 text-xs font-medium text-white border-[#222] bg-[#111] hover:bg-[#222] hover:text-white shadow-sm"
              onClick={(e) => props.button.click(e)}
            >
              {props.button.text}
            </Button>
          )}

          {props?.button2 && (
            <Button
              variant="outline"
              size="sm"
              className="h-8 px-3 text-xs font-medium text-white border-[#222] bg-[#111] hover:bg-[#222] hover:text-white shadow-sm"
              onClick={(e) => props.button2.click(e)}
            >
              {props.button2.text}
            </Button>
          )}

          {props.rows?.length >= 1 && (
            <div className="relative ml-2">
              <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-white/50" />
              <Input
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                placeholder={
                  props?.placeholder ? props.placeholder : "Search..."
                }
                className="h-8 w-[200px] pl-8 text-xs bg-[#111] border-[#222] focus-visible:ring-[#333] text-white shadow-sm"
              />
            </div>
          )}
        </div>
      </div>

      {/* private vpn servers design */}
      {props.design === "private-vpn-servers" && (
        <div className="space-y-1">
          {finalRows.length > 0 ? (
            <div className="mt-3">
              <div className="grid lg:grid-cols-2 2xl:grid-cols-3 gap-4">
                {indexes.map((ind) => renderServers(finalRows[ind], ind))}
              </div>
            </div>
          ) : (
            <div className="flex justify-center py-12 text-white/60 bg-[#0a0a0a]  rounded-lg">
              <div className="flex flex-col items-center">
                <div className="w-10 h-10 mb-3 rounded-full bg-[#111] flex items-center justify-center">
                  <Filter className="w-5 h-5 text-white/40" />
                </div>
                <p className="text-sm font-medium">No results found</p>
                {filter && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setFilter("")}
                    className="mt-3 h-7 px-2 text-xs font-medium text-blue-400 hover:bg-blue-500/10"
                  >
                    Clear Search
                  </Button>
                )}
              </div>
            </div>
          )}
        </div>
      )}

      {/* custom row */}
      {props.customRow && (
        <div>
          {[...finalRows].map((r) => (
            <>{props.customRow(r)}</>
          ))}
        </div>
      )}

      {/* regular table */}
      {!props.design && !props.customRow && (
        <div className="rounded-md  overflow-hidden bg-[#000000] shadow-lg transition-all duration-300">
          {finalRows.length > 0 && (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader className="bg-[#0a0a0a]">
                  <TableRow className="border-b border-[#222] hover:bg-transparent">
                    {props?.header?.map((l, i) => {
                      const style = {
                        ...(l.color && { color: `var(--c-${l.color})` }),
                        ...(l.align && { textAlign: l.align }),
                        ...(l.minWidth && { minWidth: l.minWidth }),
                        ...(l.width && { width: `${l.width}%` }),
                      };

                      return (
                        <TableHead
                          key={l.value + i}
                          style={style}
                          className={`py-3 px-4 text-xs font-semibold text-white/90 transition-colors uppercase tracking-wider ${l.className || ""}`}
                        >
                          {l.value}
                        </TableHead>
                      );
                    })}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {indexes.map((ind) => {
                    let r = finalRows[ind];

                    return (
                      <TableRow
                        key={"row" + ind}
                        className="border-b border-[#222] hover:bg-[#0a0a0a] transition-colors"
                      >
                        {r.items.map((i, index) => {
                          const style = {
                            ...(i.color && { color: `var(--c-${i.color})` }),
                            ...(i.align && { textAlign: i.align }),
                            ...(i.minWidth && { minWidth: i.minWidth }),
                            ...(i.width && { width: `${i.width}%` }),
                          };

                          const handleClick = i.click || (() => {});
                          const classNames = i.className || "";
                          const isClickable = i.click ? "cursor-pointer" : "";

                          let cellContent;
                          if (i.type === "text") {
                            // Enhanced status styling with badges for status-like values
                            if (
                              [
                                "Connect",
                                "Assign",
                                "tunnels",
                                "Active",
                                "Inactive",
                                "Error",
                                "Warning",
                                "Success",
                              ].includes(i.value)
                            ) {
                              let badgeColor = "";

                              if (
                                i.value === "Connect" ||
                                i.value === "Success" ||
                                i.value === "Active"
                              ) {
                                badgeColor =
                                  "bg-green-500/10 text-green-400 border-green-500/20";
                              } else if (
                                i.value === "Assign" ||
                                i.value === "tunnels"
                              ) {
                                badgeColor =
                                  "bg-blue-500/10 text-blue-400 border-blue-500/20";
                              } else if (i.value === "Error") {
                                badgeColor =
                                  "bg-red-500/10 text-red-400 border-red-500/20";
                              } else if (
                                i.value === "Warning" ||
                                i.value === "Inactive"
                              ) {
                                badgeColor =
                                  "bg-amber-500/10 text-amber-400 border-amber-500/20";
                              }

                              cellContent = (
                                <Badge
                                  className={`${badgeColor} font-medium text-xs py-0.5 px-2 border`}
                                >
                                  {i.value}
                                </Badge>
                              );
                            } else {
                              let textColorClass = "text-white/80";
                              cellContent = (
                                <div className="relative group">
                                  <span className={textColorClass}>
                                    {i.value}
                                  </span>
                                  {i.tooltip === true && (
                                    <span className="absolute left-0 top-full z-50 hidden group-hover:block bg-[#111] text-white p-2 rounded shadow-md text-xs whitespace-nowrap border border-[#333]">
                                      {i.value}
                                    </span>
                                  )}
                                </div>
                              );
                            }
                          } else if (i.type === "select") {
                            cellContent = (
                              <span className="text-white/80">{i.value}</span>
                            );
                          } else if (i.type === "img") {
                            cellContent = (
                              <div className="relative group">
                                <img
                                  src={i.value}
                                  alt="thumbnail"
                                  className="h-8 w-8 object-cover rounded-md border border-[#333]"
                                />
                                {i.tooltip && (
                                  <div className="absolute left-full top-0 ml-2 z-50 hidden group-hover:block">
                                    <img
                                      src={i.value}
                                      alt="preview"
                                      className="max-w-[200px] max-h-[150px] object-contain rounded border border-[#333] shadow-lg"
                                    />
                                  </div>
                                )}
                              </div>
                            );
                          } else if (i.type === "action") {
                            cellContent = (
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={(e) => handleClick(e)}
                                className="h-7 px-2 text-xs font-medium text-blue-400 hover:bg-blue-500/10 hover:text-blue-300"
                              >
                                {i.value}
                              </Button>
                            );
                          }

                          return (
                            <TableCell
                              key={i.value + String(index)}
                              style={style}
                              onClick={(e) => handleClick(e)}
                              className={`py-3 px-4 text-sm ${classNames} ${isClickable} ${i.click ? "hover:bg-[#111] transition-colors" : ""}`}
                            >
                              {cellContent}
                            </TableCell>
                          );
                        })}
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}

          {finalRows.length < 1 && (
            <div className="flex justify-center py-12 text-white/60">
              <div className="flex flex-col items-center">
                <div className="w-10 h-10 mb-3 rounded-full bg-[#111] flex items-center justify-center">
                  <Filter className="w-5 h-5 text-white/40" />
                </div>
                <p className="text-sm font-medium">No results found</p>
                {filter && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setFilter("")}
                    className="mt-3 h-7 px-2 text-xs font-medium text-blue-400 hover:bg-blue-500/10"
                  >
                    Clear Search
                  </Button>
                )}
              </div>
            </div>
          )}
        </div>
      )}

      {(pg.TotalPages > 1 || pg.TableSize === finalRows.length) && (
        <div className="flex items-center justify-between gap-2 mt-4 px-1">
          <div className="flex items-center text-xs text-white/50 bg-[#0a0a0a] px-3 py-1.5 rounded-md border border-[#222]">
            <span>{calculateItemsShowing()}</span>
          </div>

          <div className="flex items-center gap-3">
            <div className="flex items-center overflow-hidden rounded-md border border-[#222] shadow-sm">
              <Button
                variant="outline"
                size="icon"
                onClick={() => setPageWrap(pg.PrevPage, finalRows.length)}
                disabled={!showPP}
                className="h-8 w-8 rounded-l-md rounded-r-none border-r-0 border-[#222] bg-[#111] hover:bg-[#222] disabled:opacity-50"
              >
                <ChevronLeft className="h-4 w-4 text-white/70" />
              </Button>

              <Button
                variant="outline"
                size="icon"
                onClick={() => setPageWrap(pg.NextPage, finalRows.length)}
                disabled={!showNP}
                className="h-8 w-8 rounded-r-md rounded-l-none border-[#222] bg-[#111] hover:bg-[#222] disabled:opacity-50"
              >
                <ChevronRight className="h-4 w-4 text-white/70" />
              </Button>
            </div>

            <div className="flex items-center gap-2">
              <Select
                value={String(originalSize)}
                onValueChange={(value) =>
                  state.setPageSize(props.tableID, value)
                }
              >
                <SelectTrigger className="h-8 w-[80px] text-xs border-[#222] bg-[#111] focus:ring-0 shadow-sm">
                  <span className="text-white/50 text-xs mr-1">Items:</span>
                  <SelectValue
                    placeholder={originalSize}
                    className="text-white/90 text-xs"
                  />
                </SelectTrigger>
                <SelectContent className="bg-[#000000] border-[#222]">
                  <SelectItem
                    value="20"
                    className="text-white/90 focus:bg-[#111]"
                  >
                    20
                  </SelectItem>
                  <SelectItem
                    value="50"
                    className="text-white/90 focus:bg-[#111]"
                  >
                    50
                  </SelectItem>
                  <SelectItem
                    value="100"
                    className="text-white/90 focus:bg-[#111]"
                  >
                    100
                  </SelectItem>
                  <SelectItem
                    value="200"
                    className="text-white/90 focus:bg-[#111]"
                  >
                    200
                  </SelectItem>
                  <SelectItem
                    value="0"
                    className="text-white/90 focus:bg-[#111]"
                  >
                    All
                  </SelectItem>
                </SelectContent>
              </Select>

              <Select
                value={String(pg.CurrentPage)}
                onValueChange={(value) =>
                  setPageWrap(parseInt(value), finalRows.length)
                }
              >
                <SelectTrigger className="h-8 w-[80px] text-xs border-[#222] bg-[#111] focus:ring-0 shadow-sm">
                  <span className="text-white/50 text-xs mr-1">Page:</span>
                  <SelectValue
                    placeholder={pg.CurrentPage + 1}
                    className="text-white/90 text-xs"
                  />
                </SelectTrigger>
                <SelectContent className="bg-[#000000] border-[#222] max-h-[200px]">
                  {arrayFromTotalPages(pg.TotalPages).map((i) => (
                    <SelectItem
                      key={i}
                      value={String(i)}
                      className="text-white/90 focus:bg-[#111]"
                    >
                      {i + 1}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default NewTable;
