import { Label } from "@/components/ui/label";

const InfoItem = ({ label, value, icon }) => (
  <div className="flex items-center py-1.5 gap-3 max-w-[480px]">
    <div className="flex items-center gap-2 shrink-0 w-[120px]">
      {icon}
      <Label className="text-[12px] font-medium text-white/40">{label}</Label>
    </div>
    <code className="text-[13px] font-mono text-white/80 truncate">
      {value !== undefined && value !== null ? String(value) : "Unknown"}
    </code>
  </div>
);

export default InfoItem
