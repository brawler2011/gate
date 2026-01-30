"use client";

import { useState, useEffect } from "react";
import { IconCopy, IconCheck } from "@tabler/icons-react";
import classes from "./TruncatedWithCopy.module.css";

type Props = {
  value: string;
  showOnHover?: boolean;
  maxLength?: number;
};

export function TruncatedWithCopy({ 
  value, 
  showOnHover = false,
  maxLength = 9 
}: Props) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (copied) {
      const timer = setTimeout(() => {
        setCopied(false);
      }, 1000);
      return () => clearTimeout(timer);
    }
  }, [copied]);

  // Handle undefined, null, or non-string values
  const stringValue = value?.toString() || "";
  if (stringValue.length === 0) {
    return null;
  }

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    
    navigator.clipboard.writeText(stringValue).then(() => {
      setCopied(true);
    }).catch((err) => {
      console.error("Failed to copy:", err);
    });
  };

  const displayText = stringValue.substring(0, maxLength);

  return (
    <div className={classes.container}>
      <div 
        className={classes.textWrapper}
        style={{ width: `${maxLength * 0.55}em` }}
      >
        <span className={classes.text}>{displayText}</span>
      </div>
      <button
        className={`${classes.iconWrapper} ${showOnHover ? classes.showOnHover : ""}`}
        onClick={handleCopy}
        type="button"
        aria-label="Копировать"
      >
        {copied ? (
          <IconCheck size={16} stroke={1.7} />
        ) : (
          <IconCopy size={16} stroke={1.7} />
        )}
      </button>
    </div>
  );
}

