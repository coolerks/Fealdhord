import { useId } from 'react'

type TimeSlot = 'morning' | 'noon' | 'afternoon' | 'evening' | 'night' | 'lateNight'

const greetingsBySlot: Record<TimeSlot, readonly string[]> = {
  morning: [
    '早上好，要看看订阅里有没有新文章吗？',
    '早安，想先刷一遍未读摘要，还是搜点感兴趣的话题？',
    '早上好，今天想从哪个订阅或分类开始读？',
    '早安，需要我帮你梳理一下早间阅读内容吗？',
    '早上好，今天想找些什么文章来读？',
  ],
  noon: [
    '中午好，午休时间想快速翻翻未读文章吗？',
    '中午好，要帮你挑几篇值得细读的内容吗？',
    '中午好，想围绕某个主题在订阅里找点文章吗？',
    '中午好，想随手读点轻松的，还是整理一下未读摘要？',
    '中午好，有什么文章或话题想让我帮你找找吗？',
  ],
  afternoon: [
    '下午好，想查点订阅里的文章，还是聊聊刚读到的内容？',
    '下午好，需要帮你总结一下某个分类或主题的文章吗？',
    '下午好，今天有什么感兴趣的方向想深入搜一搜？',
    '下午好，要看看精选里有哪些值得一读的文章吗？',
    '下午好，有什么想快速定位或对照阅读的文章吗？',
  ],
  evening: [
    '傍晚好，需要我帮你把今天的阅读内容整理成摘要吗？',
    '傍晚好，今天读到的文章里，有想继续深挖的话题吗？',
    '傍晚好，想清理一下未读列表，还是看看精选文章？',
    '傍晚好，需要我按主题帮你梳理一下最近的文章吗？',
    '傍晚好，想回顾一下今天关注过的内容吗？',
  ],
  night: [
    '晚上好，今晚想挑几篇长文慢慢读，还是先看摘要？',
    '晚上好，今天有哪些收藏或文章想一起聊聊？',
    '晚上好，想复盘一下今天的阅读收获，还是随便逛逛订阅？',
    '晚上好，需要帮你从订阅里找些感兴趣的内容吗？',
    '晚上好，今晚准备读点什么呢？',
  ],
  lateNight: [
    '夜深了，想安静翻几篇好文章，还是快速扫一眼未读？',
    '夜深了，有什么想顺手查一下的文章或线索吗？',
    '这么晚还在阅读，需要我帮你提炼一下重点吗？',
    '夜深人静，有哪篇文章的想法想一起聊聊吗？',
    '夜深了，想给今天的阅读简单收个尾吗？',
  ],
}

export function timeSlotForHour(hour: number): TimeSlot {
  if (hour >= 5 && hour < 11) return 'morning'
  if (hour >= 11 && hour < 14) return 'noon'
  if (hour >= 14 && hour < 18) return 'afternoon'
  if (hour >= 18 && hour < 20) return 'evening'
  if (hour >= 20 && hour < 23) return 'night'
  return 'lateNight'
}

export function pickGreeting(now = new Date(), random = Math.random): string {
  const options = greetingsBySlot[timeSlotForHour(now.getHours())]
  const index = Math.min(options.length - 1, Math.floor(random() * options.length))
  return options[index]
}

/** 提取自 build/appicon.svg 的三层悬浮折页图形（不含外层圆角矩形底板）。 */
export function AppMark({ className }: { className?: string }) {
  const prefix = useId().replace(/:/g, '')
  const id = (name: string) => `${prefix}-${name}`

  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="240 230 700 680"
      className={className}
      aria-hidden="true"
    >
      <defs>
        <filter id={id('markShadow')} x="-22%" y="-30%" width="150%" height="172%" colorInterpolationFilters="sRGB">
          <feGaussianBlur in="SourceAlpha" stdDeviation="15" result="blur" />
          <feOffset in="blur" dx="0" dy="17" result="offset" />
          <feFlood floodColor="#0A1838" floodOpacity=".12" result="tone" />
          <feComposite in="tone" in2="offset" operator="in" result="shadow" />
          <feMerge>
            <feMergeNode in="shadow" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
        <filter id={id('upperLeafShadow')} x="-24%" y="-24%" width="150%" height="155%" colorInterpolationFilters="sRGB">
          <feGaussianBlur in="SourceAlpha" stdDeviation="6" result="blur" />
          <feOffset in="blur" dx="2" dy="6" result="offset" />
          <feFlood floodColor="#233A6E" floodOpacity=".18" result="tone" />
          <feComposite in="tone" in2="offset" operator="in" result="shadow" />
          <feMerge>
            <feMergeNode in="shadow" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
        <linearGradient id={id('bottomLeaf')} x1="337" y1="665" x2="835" y2="869" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#102852" />
          <stop offset=".36" stopColor="#142E5E" />
          <stop offset=".75" stopColor="#24529C" />
          <stop offset="1" stopColor="#2E61AD" />
        </linearGradient>
        <radialGradient id={id('bottomSheen')} cx="0" cy="0" r="1" gradientTransform="translate(547 866) rotate(-68) scale(272 375)" gradientUnits="userSpaceOnUse">
          <stop stopColor="#2C61B3" stopOpacity=".52" />
          <stop offset="1" stopColor="#2C61B3" stopOpacity="0" />
        </radialGradient>
        <linearGradient id={id('middleLeaf')} x1="292" y1="698" x2="862" y2="481" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#416FAC" />
          <stop offset=".2" stopColor="#7398CD" />
          <stop offset=".44" stopColor="#A5C3F3" />
          <stop offset=".75" stopColor="#C7DBFF" />
          <stop offset="1" stopColor="#A8C4F4" />
        </linearGradient>
        <radialGradient id={id('middleSoftLight')} cx="0" cy="0" r="1" gradientTransform="translate(732 666) rotate(158) scale(390 210)" gradientUnits="userSpaceOnUse">
          <stop stopColor="#E3EEFF" stopOpacity=".65" />
          <stop offset="1" stopColor="#E3EEFF" stopOpacity="0" />
        </radialGradient>
        <linearGradient id={id('upperLeaf')} x1="465" y1="300" x2="616" y2="661" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#81A9E0" />
          <stop offset=".52" stopColor="#A2C2EF" />
          <stop offset=".85" stopColor="#B3CEF3" />
          <stop offset="1" stopColor="#A1BFE8" />
        </linearGradient>
        <linearGradient id={id('upperEdge')} x1="860" y1="255" x2="748" y2="450" gradientUnits="userSpaceOnUse">
          <stop stopColor="#D9DFED" stopOpacity=".72" />
          <stop offset=".35" stopColor="#CFD6E8" stopOpacity=".32" />
          <stop offset="1" stopColor="#7F98C4" stopOpacity=".08" />
        </linearGradient>
        <path id={id('bottomShape')} d="M268 754 C342 701 413 650 490 649 C579 647 650 671 712 698 C776 727 824 688 865 632 C895 644 906 659 900 683 C886 724 859 783 838 830 C820 877 785 889 742 880 C662 865 574 808 495 776 C415 743 338 741 268 757 Z" />
        <path id={id('middleShape')} d="M268 754 C294 671 370 605 486 565 C610 530 699 496 748 451 C787 466 849 486 892 499 C909 504 919 520 912 541 L867 632 C846 681 828 709 802 716 C766 729 726 718 682 705 C586 676 518 664 459 671 C381 677 311 717 268 754 Z" />
        <path id={id('topShape')} d="M268 753 C282 641 324 535 387 446 C477 322 652 281 868 254 C830 312 786 390 748 451 C716 504 680 529 600 551 C479 584 378 612 313 682 C291 706 276 729 268 753 Z" />
      </defs>
      <g filter={`url(#${id('markShadow')})`}>
        <use href={`#${id('bottomShape')}`} fill={`url(#${id('bottomLeaf')})`} />
        <use href={`#${id('bottomShape')}`} fill={`url(#${id('bottomSheen')})`} />
        <path d="M268 754 C349 696 422 650 491 649 C582 646 647 675 715 700" fill="none" stroke="#355B9B" strokeOpacity=".45" strokeWidth="2.4" />
        <use href={`#${id('middleShape')}`} fill={`url(#${id('middleLeaf')})`} />
        <use href={`#${id('middleShape')}`} fill={`url(#${id('middleSoftLight')})`} />
        <g filter={`url(#${id('upperLeafShadow')})`}>
          <use href={`#${id('topShape')}`} fill={`url(#${id('upperLeaf')})`} />
        </g>
        <path d="M868 254 C830 313 787 389 748 451" stroke={`url(#${id('upperEdge')})`} strokeWidth="1.25" fill="none" strokeLinecap="round" />
        <path d="M268 753 C282 641 324 535 387 446 C477 322 652 281 868 254" fill="none" stroke="#577FB6" strokeWidth="1.1" strokeOpacity=".72" />
      </g>
    </svg>
  )
}
