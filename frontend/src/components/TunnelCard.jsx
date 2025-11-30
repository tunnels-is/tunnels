import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  MoreHorizontal,
  Edit,
  Trash2,
  Copy,
  Network,
  Server,
  Shield,
} from "lucide-react";
import { toast } from "sonner";
import { GetEncType } from "@/lib/helpers";


const TunnelCard = ({ tunnel, onConnect, onEdit, onDelete }) => {
  const copyToClipboard = (text) => {
    navigator.clipboard.writeText(text);
    toast.success("Copied to clipboard");
  };

  return (
    <Card className="border-[#1a1f2d] text-white hover:border-[#2a3142] transition-colors">
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-lg font-bold flex items-center gap-2">
          <Network className="w-5 h-5 text-blue-400" />
          {tunnel.Tag}
        </CardTitle>
        <Badge
          variant="outline"
          className="bg-[#2a1db5] hover:bg-white hover:text-black text-white"
        >
          {GetEncType(tunnel.EncryptionType)}
        </Badge>
      </CardHeader>
      <CardContent>
        <div className="grid gap-4 py-4">
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <Server className="w-4 h-4" />
            <span>Server ID: {tunnel.ServerID}</span>
          </div>
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <Shield className="w-4 h-4" />
            <span>Interface: {tunnel.IFName}</span>
          </div>
          <div className="space-y-2">
            <div className="flex items-center justify-between bg-[#151a25] p-2 rounded text-xs font-mono">
              <span className="text-gray-300">IPv4: {tunnel.IPv4Address}</span>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={() => copyToClipboard(tunnel.IPv4Address)}
              >
                <Copy className="w-3 h-3" />
              </Button>
            </div>
            <div className="flex items-center justify-between bg-[#151a25] p-2 rounded text-xs font-mono">
              <span className="text-gray-300">IPv6: {tunnel.IPv6Address}</span>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={() => copyToClipboard(tunnel.IPv6Address)}
              >
                <Copy className="w-3 h-3" />
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
      <CardFooter className="flex justify-between">
        {onConnect && onConnect(tunnel)}
        <div className="flex gap-2">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onEdit(tunnel)}
            className="h-8 w-8 text-gray-400 hover:text-white"
          >
            <Edit className="w-4 h-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onDelete(tunnel)}
            className="h-8 w-8 text-red-500 hover:text-red-400 hover:bg-red-950/20"
          >
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      </CardFooter>
    </Card>
  );
};

export default TunnelCard;
