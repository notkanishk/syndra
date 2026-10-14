import { PageHeader, MetaRow, Mono, Button, ButtonLink } from "ui";

export const WithActions = () => (
  <PageHeader
    eyebrow="Projects"
    title="Fabrication Lab"
    lede="The roles this project offers, the apps that check them, and who holds each."
    meta="48 people · 6 roles · serves Door access and Booking"
    actions={
      <>
        <ButtonLink href="/projects/fab-lab/roles/new" variant="outline">Add role</ButtonLink>
        <Button variant="accent">Grant access</Button>
      </>
    }
  />
);

/** A person's page: meta is a dot-separated MetaRow, never a sentence. */
export const PersonMeta = () => (
  <PageHeader
    title="Aanya Rao"
    lede="Everything this person can use, and where each piece of access came from. Change it here; Syndra keeps the record of why."
    meta={
      <MetaRow>
        {[
          "aanya.rao_ug25@example.edu",
          "Student lead",
          "Robotics club",
          <Mono key="id">312874519023645698</Mono>,
        ]}
      </MetaRow>
    }
  />
);

export const TitleAndLede = () => (
  <PageHeader
    title="Expiring access"
    lede="Access that ends in the next 30 days. If nobody renews it, it ends on its own and the person loses the door or machine."
  />
);
