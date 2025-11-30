import { useQuery } from "@tanstack/react-query";
import { getDNSStats } from "../api/dns";

export const useDNSStats = () => {
    return useQuery({
        queryKey: ["dns-stats"],
        queryFn: getDNSStats,
        refetchInterval: 5000, // Refresh every 5 seconds as it's stats
    });
};
