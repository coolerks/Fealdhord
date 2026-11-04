// 来源：lucide-animated（MIT），https://github.com/pqoqubbw/icons
"use client";

import { motion, useAnimation } from "motion/react";
import type { HTMLAttributes } from "react";
import { forwardRef, useCallback, useImperativeHandle, useRef } from "react";

import { cn } from "@/lib/utils";

export interface BookTextIconHandle {
  startAnimation: () => void;
  stopAnimation: () => void;
}

interface BookTextIconProps extends HTMLAttributes<HTMLDivElement> {
  size?: number;
}

const BookTextIcon = forwardRef<BookTextIconHandle, BookTextIconProps>(
  ({ onMouseEnter, onMouseLeave, className, size = 28, ...props }, ref) => {
    const controls = useAnimation();
    const isControlledRef = useRef(false);

    useImperativeHandle(ref, () => {
      isControlledRef.current = true;

      return {
        startAnimation: () => controls.start("animate"),
        stopAnimation: () => controls.start("normal"),
      };
    });

    const handleMouseEnter = useCallback(
      (e: React.MouseEvent<HTMLDivElement>) => {
        if (isControlledRef.current) {
          onMouseEnter?.(e);
        } else {
          controls.start("animate");
        }
      },
      [controls, onMouseEnter]
    );

    const handleMouseLeave = useCallback(
      (e: React.MouseEvent<HTMLDivElement>) => {
        if (isControlledRef.current) {
          onMouseLeave?.(e);
        } else {
          controls.start("normal");
        }
      },
      [controls, onMouseLeave]
    );

    return (
      <div
        className={cn(className)}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        {...props}
      >
        <svg fill="none" height={size} width={size} stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2"
          viewBox="0 0 24 24"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H19a1 1 0 0 1 1 1v18a1 1 0 0 1-1 1H6.5a1 1 0 0 1 0-5H20" />
          <motion.path animate={controls} initial="normal" variants={{normal:{pathLength:1,opacity:1},animate:{pathLength:[0,1],opacity:[0,1],transition:{duration:.5,delay:.15}}}} d="M8 11h8" />
          <motion.path animate={controls} initial="normal" variants={{normal:{pathLength:1,opacity:1},animate:{pathLength:[0,1],opacity:[0,1],transition:{duration:.5}}}} d="M8 7h6" />
        </svg>
      </div>
    );
  }
);

BookTextIcon.displayName = "BookTextIcon";

export { BookTextIcon };
