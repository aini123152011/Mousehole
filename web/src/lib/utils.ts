import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/** 合并基础控件样式，使用调用方最后提供的 Tailwind 规则。 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
