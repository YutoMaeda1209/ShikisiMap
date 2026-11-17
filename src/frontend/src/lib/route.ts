// The site has two pages, so the path is matched by hand instead of with a
// router library.

export type Route = { page: "map" } | { page: "profile"; userId: string };

const profilePath = /^\/users\/([0-9a-fA-F-]{36})\/?$/;

export function parseRoute(pathname: string): Route {
  const match = pathname.match(profilePath);
  return match ? { page: "profile", userId: match[1].toLowerCase() } : { page: "map" };
}

export function profileUrl(userId: string): string {
  return `${window.location.origin}/users/${userId}`;
}
