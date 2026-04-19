"use client";

import { useEffect, useRef, useState, type CSSProperties } from "react";

export interface ChainNameProps {
  name: string;
}

// ChainName: 链路卡片里的 handler 名字标签. 如果文本宽度超过容器, 在
// CSS 层用 keyframes 做 "水平滚动" 动画 (无限循环回到起点), 避免长
// 名字被直接 ellipsis 截断让用户看不到完整内容.
//
// 测量逻辑: scrollWidth 是 "完整文本" 宽度, clientWidth 是 "可见
// 容器" 宽度. 差值 distance 作为 CSS variable 传给 keyframes, 用来
// 算平移距离. distance === 0 表示不溢出, 直接不加 scroll class.
//
// 只在 name 变化时重测. 容器宽度变化 (比如窗口 resize) 目前不会触发
// 重测, 但这是低频场景, 等出现具体 bug 再引入 ResizeObserver.
export function ChainName({ name }: ChainNameProps) {
  const wrapRef = useRef<HTMLSpanElement | null>(null);
  const textRef = useRef<HTMLSpanElement | null>(null);
  const [distance, setDistance] = useState(0);

  useEffect(() => {
    const wrap = wrapRef.current;
    const text = textRef.current;
    if (!wrap || !text) {
      return;
    }
    const nextDistance = Math.max(0, text.scrollWidth - wrap.clientWidth);
    setDistance(nextDistance);
  }, [name]);

  return (
    <span className="handler-debug-chain-name-wrap" ref={wrapRef} title={name}>
      <span
        ref={textRef}
        className={`handler-debug-chain-name ${distance > 0 ? "handler-debug-chain-name-scroll" : ""}`}
        style={distance > 0 ? ({ "--handler-chain-scroll-distance": `${distance}px` } as CSSProperties) : undefined}
      >
        {name}
      </span>
    </span>
  );
}
