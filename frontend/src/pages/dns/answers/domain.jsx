import React from "react";
import { useParams } from "react-router-dom";
import { toast } from "sonner";
import { getDNSStats } from "@/api/dns";


export default function DNSAnswers() {
  const { data: dnsStats } = useQuery({
    queryKey: ["dns-stats"],
    queryFn: getDNSStats,
    refetchInterval: 5000, // Refresh every 5 seconds as it's stats
  });

  const { domain } = useParams()

  const OpenWindowURL = (baseurl, value) => {
    let final = baseurl + value
    window.open(final, "_blank")
    try {
      if (navigator?.clipboard) {
        navigator.clipboard.writeText(final);
        toast.success("Link copied to clipboard");
      }
    } catch (e) {
      console.log(e)
    }
  }

  let answers = new Map();

  if (dnsStats && dnsStats[domain] && dnsStats[domain].Answers) {
    dnsStats[domain].Answers.map(a => {
      let as = a.split("\t")
      let children = []
      as.forEach((a, index) => {
        let isLast = false
        let isFirst = false
        let classes = "column"
        let baseURL = "https://whois.com/whois/"
        if (index === as.length - 1) {
          isLast = true
          classes += " bold"
        } else if (index === 0) {
          isFirst = true
          classes += " cblue  cursor"
        } else {
          classes += " dimmed"
        }
        if (a === "A" || a === "AAAA") {
          classes += " cgreen"
        } else if (a === "CNAME") {
          classes += " cgreen"
        }
        if (isFirst) {
          let urlvalue = ""
          if (isFirst) {
            let ds = a.split(".")
            urlvalue = ds[ds.length - 3] + "." + ds[ds.length - 2]
          }

          children.push(<div className={classes} onClick={() => {
            OpenWindowURL(baseURL, urlvalue);
          }}>{a}</div>)
        } else {
          children.push(<div className={classes}>{a}</div>)
        }
      })
      let e = <div className="answer">{children}</div>
      let key = as[4] + " " + as[2] + " " + as[3] + " " + as[0]
      let x = answers.get(key)
      if (!x) {
        answers.set(key, e)
      }
      return
    })
  }

  return (
    <div className={className}>
      {answers.size < 1 &&
        <div className="title">no records found</div>
      }
      {Array.from(answers.entries()).map(a => {
        if (a.length > 1) {
          return (a[1])
        } else {
          return (a[0])
        }
      })}
    </div >
  )
}
