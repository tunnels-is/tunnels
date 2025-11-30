import React from "react";
import { SupportPlatforms } from "../lib/constants";

import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ExternalLink } from "lucide-react";

const Welcome = () => {

  const Copy = (value) => {
    window.open(value, "_blank");
  };

  return (
    <div className="p-6 space-y-6">
      <div className="grid gap-4">
        {SupportPlatforms.map((s) => (
          <Card key={s.name} className="hover:shadow-md transition">
            <CardContent className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 p-4">
              <div className="space-y-1">
                <div
                  onClick={() => Copy(s.link)}
                  className="text-lg font-medium cursor-pointer hover:underline flex items-center gap-2"
                >
                  {s.name} <ExternalLink size={16} />
                </div>

                {s.type === "email" && (
                  <div className="text-sm text-muted-foreground">
                    <a href={`mailto:${s.link}`} className="underline">
                      {s.link}
                    </a>
                  </div>
                )}
                {s.type === "link" && (
                  <div className="text-sm text-muted-foreground">
                    <a
                      href={s.link}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="underline"
                    >
                      {s.link}
                    </a>
                  </div>
                )}
              </div>

              <Button
                size="sm"
                onClick={() => Copy(s.link)}
                className="w-full sm:w-auto"
              >
                Open
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
};

export default Welcome;
