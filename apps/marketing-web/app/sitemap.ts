import type { MetadataRoute } from "next";
import { siteUrl } from "./env";
import { featurePages, solutionPages } from "../lib/marketing-pages";

const routes = [
  "",
  "/fr",
  "/product",
  "/features",
  ...featurePages.map((page) => `/features/${page.slug}`),
  "/solutions",
  ...solutionPages.map((page) => `/solutions/${page.slug}`),
  "/integrations",
  "/resources",
  "/templates",
  "/status",
  "/pricing",
  "/demo",
  "/signup",
  "/privacy",
  "/dpa",
  "/terms",
  "/security",
  "/contact",
];

export default function sitemap(): MetadataRoute.Sitemap {
  return routes.map((route) => ({
    url: `${siteUrl}${route}`,
    lastModified: new Date(),
    changeFrequency: route === "" ? "weekly" : "monthly",
    priority: route === "" ? 1 : 0.8,
  }));
}
