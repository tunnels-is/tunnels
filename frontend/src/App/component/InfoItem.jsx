import { Label } from "@/components/ui/label";

const InfoItem = ({ label, value, icon }) => (
  <div className="flex flex-col py-1 space-y-1">
    <div className="flex items-center gap-2">
      {icon}
      <Label className="text-sm font-medium">{label}</Label>
    </div>
    <code className="text-md block font-mono bg-muted/60 px-2 py-1.5 h-9  w-full overflow-hidden  break-all text-ellipsis text-nowrap">
      {value !== undefined && value !== null ? String(value) : "Unknown"}
    </code>
  </div>
);

export default InfoItem
