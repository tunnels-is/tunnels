import React, { useState, useEffect } from "react";
import { useAtomValue } from "jotai";
import { userAtom } from "@/stores/userStore";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Shield,
  Key,
  CreditCard,
  Server,
  Copy,
  Check,
  Mail,
  Lock,
  Crown,
  QrCode
} from "lucide-react";
import dayjs from "dayjs";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import QRCode from "react-qr-code";
import { client, forwardToController } from "@/api/client";

const TwoFactorDialog = ({ user }) => {
  const [inputs, setInputs] = useState({});
  const [errors, setErrors] = useState({});
  const [code, setCode] = useState({});
  const [isOpen, setIsOpen] = useState(false);

  const HandleInputChange = (event) => {
    setInputs(inputs => ({ ...inputs, [event.target.name]: event.target.value }));
  };

  const Get2FACode = async () => {
    if (!user) return;
    setInputs({});
    setErrors({});
    setCode({}); // Reset code state

    let data = { Email: user.Email };
    try {
      const response = await client.post("/getQRCode", data);
      if (response?.data) {
        setCode(response.data);
      } else {
        toast.error("Unknown error, please try again in a moment");
        setIsOpen(false);
      }
    } catch (e) {
      toast.error("Failed to get QR code");
      setIsOpen(false);
    }
  };

  const HandleSubmit = async () => {
    let newErrors = {};
    let hasErrors = false;

    if (!inputs["digits"]) {
      newErrors["digits"] = "Authenticator code missing";
      hasErrors = true;
    } else if (inputs["digits"].length !== 6) {
      newErrors["digits"] = "Authenticator code must be 6 digits";
      hasErrors = true;
    }

    if (!inputs["password"]) {
      newErrors["password"] = "Please enter your password";
      hasErrors = true;
    }

    if (hasErrors) {
      setErrors(newErrors);
      return;
    }

    // Logic from Enable2FA.jsx
    // The QR string format might be otpauth://totp/...?secret=SECRET&...
    // We need to extract the secret if the API expects it separately, 
    // OR the API might just need the password and 'digits' (code).
    // Looking at Enable2FA.jsx:
    // let c = code.Value
    // let firstSplit = c.split("&")
    // let secondSPlit = firstSplit[1].split("=")
    // let secret = secondSPlit[1]
    // inputs.Code = secret

    // Note: This parsing logic seems fragile and depends on the text structure of code.Value.
    // I will replicate it carefully.

    let secret = "";
    if (code.Value) {
      try {
        let c = code.Value;
        let firstSplit = c.split("&"); // Check if this exists
        if (firstSplit.length > 1) {
          let secondSplit = firstSplit[1].split("=");
          if (secondSplit.length > 1) {
            secret = secondSplit[1];
          }
        }
      } catch (err) {
        console.error("Error parsing secret", err);
      }
    }

    if (secret === "") {
      // Fallback or error if secret isn't found this way. 
      // However, Enable2FA.jsx just toasts.
      // If the API endpoint /v3/user/2fa/confirm actually needs the 'secret' passed as 'Code', 
      // we must ensure we get it.
      // Let's assume code.Value IS the otpauth URL.
      // otpauth://totp/Label?secret=SECRET&issuer=Issuer

      // Safer parsing:
      try {
        const url = new URL(code.Value);
        secret = url.searchParams.get("secret");
      } catch (e) {
        // Fallback to the original split method if URL parsing fails 
        // (though otpauth URI should be parsable if valid)
        // or just let it fail below.

        let c = code.Value || "";
        let firstSplit = c.split("&");
        if (firstSplit[1]) {
          let secondSPlit = firstSplit[1].split("=");
          if (secondSPlit[1]) secret = secondSPlit[1];
        }
      }
    }

    if (!secret) {
      toast.error("Could not parse authenticator secret from QR Response");
      return;
    }

    inputs.Code = secret;

    try {
      const response = await forwardToController("POST", "/v3/user/2fa/confirm", inputs);
      if (response.status === 200) {
        let c = { ...code };
        c.Recovery = response.data.Data;
        setCode(c);
        toast.success("2FA Enabled Successfully!");
      }
    } catch (e) {
      toast.error("Failed to confirm 2FA");
      console.error(e);
    }
    setErrors({});
  };

  return (
    <Dialog open={isOpen} onOpenChange={(open) => {
      setIsOpen(open);
      if (open) Get2FACode();
    }}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm" className={user?.TwoFactorEnabled ? "hidden" : "gap-2"}>
          <QrCode className="h-4 w-4" /> Enable 2FA
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md bg-[#0B0E14] border-[#1a1f2d] text-white">
        <DialogHeader>
          <DialogTitle>Enable Two-Factor Authentication</DialogTitle>
          <DialogDescription>
            Scan the QR code with your authenticator app to get started.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col items-center justify-center space-y-6 py-4">
          {code.Value && !code.Recovery && (
            <>
              <div className="p-4 bg-white rounded-lg">
                <QRCode
                  style={{ height: "auto", maxWidth: "200px", width: "200px" }}
                  value={code.Value}
                  viewBox={`0 0 256 256`}
                />
              </div>

              <div className="w-full space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="password">Password</Label>
                  <div className="relative">
                    <Lock className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                      id="password"
                      name="password"
                      type="password"
                      placeholder="Confirm your password"
                      className="pl-9 bg-[#151a25] border-[#2a3142]"
                      value={inputs["password"] || ""}
                      onChange={HandleInputChange}
                    />
                  </div>
                  {errors["password"] && <p className="text-sm text-red-500">{errors["password"]}</p>}
                </div>

                <div className="space-y-2">
                  <Label htmlFor="digits">Authenticator Code</Label>
                  <div className="relative">
                    <Shield className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                      id="digits"
                      name="digits"
                      type="text"
                      placeholder="123456"
                      className="pl-9 bg-[#151a25] border-[#2a3142]"
                      value={inputs["digits"] || ""}
                      onChange={HandleInputChange}
                    />
                  </div>
                  {errors["digits"] && <p className="text-sm text-red-500">{errors["digits"]}</p>}
                </div>

                <Button className="w-full bg-blue-600 hover:bg-blue-700" onClick={HandleSubmit}>
                  Verify & Enable
                </Button>

                <div className="space-y-2">
                  <Label htmlFor="recovery" className="text-muted-foreground text-xs">Correction/Recovery Override (Optional)</Label>
                  <Input
                    id="recovery"
                    name="recovery"
                    placeholder="Recovery Code (if overriding)"
                    className="bg-[#151a25] border-[#2a3142] text-xs h-8"
                    value={inputs["recovery"] || ""}
                    onChange={HandleInputChange}
                  />
                </div>
              </div>
            </>
          )}

          {code.Recovery && (
            <div className="w-full space-y-4 text-center">
              <div className="flex flex-col items-center justify-center p-4 bg-green-500/10 text-green-500 rounded-full w-16 h-16 mx-auto mb-2">
                <Check className="h-8 w-8" />
              </div>
              <h3 className="text-lg font-bold text-white">2FA Enabled!</h3>

              <div className="text-left bg-[#151a25] border border-[#2a3142] p-4 rounded-md">
                <p className="text-sm text-red-400 font-bold mb-2">SAVE THESE RECOVERY CODES:</p>
                <pre className="text-xs whitespace-pre-wrap font-mono select-all text-white">
                  {code.Recovery}
                </pre>
              </div>
              <p className="text-sm text-muted-foreground">
                Do not store these codes with your password.
              </p>

              <Button className="w-full" variant="secondary" onClick={() => setIsOpen(false)}>
                Close
              </Button>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
};

export default function ProfilePage() {
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
    <div className="w-full mt-16 space-y-6 mx-auto pb-10">
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
                <div className="flex items-center gap-2">
                  <Badge variant={user.TwoFactorEnabled ? "default" : "secondary"} className={user.TwoFactorEnabled ? "bg-green-600 hover:bg-green-700" : ""}>
                    {user.TwoFactorEnabled ? "Enabled" : "Disabled"}
                  </Badge>
                  {!user.TwoFactorEnabled && <TwoFactorDialog user={user} />}
                </div>
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
