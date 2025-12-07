import React, { useState } from "react";
import { useAtomValue } from "jotai";
import { userAtom } from "@/stores/userStore";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import {
  User,
  Shield,
  Key,
  CreditCard,
  Server,
  Copy,
  Check,
  Mail,
  Calendar,
  Lock,
  Crown
} from "lucide-react";
import dayjs from "dayjs";
import { toast } from "sonner";

export default function Profile() {
  const user = useAtomValue(userAtom);
  const [copied, setCopied] = useState(false);

  const copyToClipboard = (text) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    toast.success("API Key copied to clipboard");
    setTimeout(() => setCopied(false), 2000);
  };

  if (!user) return null;

  const initials = user.Email
    ? user.Email.substring(0, 2).toUpperCase()
    : "U";

  const formatDate = (date) => {
    if (!date) return "N/A";
    return dayjs(date).format("MMMM D, YYYY");
  };

  const getSubStatusColor = (status) => {
    switch (status?.toLowerCase()) {
      case "active": return "bg-green-500/10 text-green-500 hover:bg-green-500/20";
      case "trial": return "bg-amber-500/10 text-amber-500 hover:bg-amber-500/20";
      case "expired": return "bg-red-500/10 text-red-500 hover:bg-red-500/20";
      default: return "bg-gray-500/10 text-gray-500";
    }
  };

  return (
    <div className="w-full mt-16 space-y-6 max-w-5xl mx-auto pb-10">
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white mb-1">My Profile</h1>
          <p className="text-muted-foreground">Manage your account settings and preferences.</p>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* User Identity Card */}
        <Card className="md:col-span-1 bg-[#0B0E14] border-[#1a1f2d]">
          <CardHeader className="text-center pb-2">
            <div className="mx-auto mb-4 relative">
              <div className="h-24 w-24 rounded-full bg-gradient-to-br from-blue-600 to-indigo-700 flex items-center justify-center text-3xl font-bold text-white shadow-xl ring-4 ring-[#0B0E14]">
                {initials}
              </div>
              {user.IsAdmin && (
                <div className="absolute -top-1 -right-1 bg-amber-500 text-black p-1.5 rounded-full shadow-lg" title="Administrator">
                  <Crown className="h-4 w-4 fill-current" />
                </div>
              )}
            </div>
            <CardTitle className="text-xl">{user.Name || "User"}</CardTitle>
            <CardDescription className="flex items-center justify-center gap-1.5 mt-1">
              <Mail className="h-3 w-3" />
              {user.Email}
            </CardDescription>
            <div className="flex flex-wrap justify-center gap-2 mt-4">
              {user.IsAdmin && <Badge variant="outline" className="border-amber-500/50 text-amber-500">Administrator</Badge>}
              {user.IsManager && <Badge variant="outline" className="border-blue-500/50 text-blue-500">Manager</Badge>}
              {!user.IsAdmin && !user.IsManager && <Badge variant="outline">User</Badge>}
            </div>
          </CardHeader>
          <CardContent className="mt-4">
            <div className="space-y-4">
              <div className="flex items-center justify-between text-sm p-3 bg-[#151a25] rounded-lg border border-[#2a3142]">
                <span className="text-muted-foreground">Member Since</span>
                <span className="font-medium text-white">{formatDate(user.Created || new Date())}</span>
              </div>
              <div className="flex items-center justify-between text-sm p-3 bg-[#151a25] rounded-lg border border-[#2a3142]">
                <span className="text-muted-foreground flex items-center gap-2">
                  <Shield className="h-3.5 w-3.5" /> 2FA Status
                </span>
                <Badge variant={user.TwoFactorEnabled ? "default" : "secondary"} className={user.TwoFactorEnabled ? "bg-green-600 hover:bg-green-700" : ""}>
                  {user.TwoFactorEnabled ? "Enabled" : "Disabled"}
                </Badge>
              </div>
            </div>
          </CardContent>
        </Card>

        <div className="md:col-span-2 space-y-6">
          {/* Control Server Info */}
          <Card className="bg-[#0B0E14] border-[#1a1f2d]">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg">
                <Server className="h-5 w-5 text-blue-500" />
                Control Server
              </CardTitle>
              <CardDescription>The server managing your authentication and config.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-1">
                  <Label className="text-xs text-muted-foreground uppercase tracking-wider">Host</Label>
                  <div className="font-mono text-sm p-2 bg-[#151a25] rounded border border-[#2a3142] text-white">
                    {user.ControlServer?.Host || "N/A"}
                  </div>
                </div>
                <div className="space-y-1">
                  <Label className="text-xs text-muted-foreground uppercase tracking-wider">Port</Label>
                  <div className="font-mono text-sm p-2 bg-[#151a25] rounded border border-[#2a3142] text-white">
                    {user.ControlServer?.Port || "N/A"}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* API Key */}
          <Card className="bg-[#0B0E14] border-[#1a1f2d]">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg">
                <Key className="h-5 w-5 text-amber-500" />
                API Access
              </CardTitle>
              <CardDescription>Your personal API key for external access.</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-2">
                <Label>API Key</Label>
                <div className="flex gap-2">
                  <div className="relative flex-1">
                    <Input
                      readOnly
                      type="password"
                      value={user.APIKey || "********************************"}
                      className="pr-10 bg-[#151a25] border-[#2a3142] font-mono"
                    />
                    <div className="absolute inset-y-0 right-0 flex items-center pr-3 pointer-events-none text-muted-foreground">
                      <Lock className="h-4 w-4" />
                    </div>
                  </div>
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => copyToClipboard(user.APIKey || "")}
                    className="border-[#2a3142] hover:bg-[#2a3142]"
                  >
                    {copied ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground mt-1">
                  Keep this key secret. It grants full access to your account.
                </p>
              </div>
            </CardContent>
          </Card>

          {/* Subscription Info */}
          <Card className="bg-[#0B0E14] border-[#1a1f2d]">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg">
                <CreditCard className="h-5 w-5 text-green-500" />
                Subscription
              </CardTitle>
              <CardDescription>Current plan details and status.</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div className="p-4 rounded-lg bg-[#151a25] border border-[#2a3142] flex flex-col items-center justify-center gap-1">
                  <span className="text-xs text-muted-foreground">Current Plan</span>
                  <span className="text-lg font-bold text-white">{user.SubPlan || "Free"}</span>
                </div>
                <div className="p-4 rounded-lg bg-[#151a25] border border-[#2a3142] flex flex-col items-center justify-center gap-1">
                  <span className="text-xs text-muted-foreground">Status</span>
                  <Badge variant="secondary" className={`mt-1 ${getSubStatusColor(user.SubStatus || (user.Trial ? "trial" : "active"))}`}>
                    {(user.SubStatus || (user.Trial ? "Trial" : "Active")).toUpperCase()}
                  </Badge>
                </div>
                <div className="p-4 rounded-lg bg-[#151a25] border border-[#2a3142] flex flex-col items-center justify-center gap-1">
                  <span className="text-xs text-muted-foreground">Expires On</span>
                  <span className="font-medium text-white text-center text-sm">{formatDate(user.SubExpiration) === "N/A" ? "Never" : formatDate(user.SubExpiration)}</span>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}