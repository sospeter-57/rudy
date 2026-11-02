import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import {
  Check,
  DoorOpen,
  Hash,
  ImagePlus,
  Lock,
  MessageSquare,
  MessageSquarePlus,
  Plus,
  Search,
  Send,
  Trash2,
  UserPlus,
  Users,
  X,
} from 'lucide-react'
import { api } from '@/lib/api'
import { useChat, useChatMessages, useChatRequests, useChats, useDiscoverChats, useMe, useUsers } from '@/lib/hooks'
import { useSession } from '@/lib/session'
import { relativeDate } from '@/lib/format'
import { toast } from '@/lib/toast'
import { cx } from '@/lib/cx'
import { Avatar, Badge, Button, Dialog, EmptyState, Field, Input, Select, Spinner, Textarea } from '@/components/ui'
import type { ChatAttachment, ChatMessage, ChatRequest, ChatRoom, RoomVisibility, User } from '@/lib/types'

function roomTitle(room: ChatRoom, viewerID: number): string {
  if (room.kind === 'dm') {
    const other = room.members?.find((member) => member.userId !== viewerID)
    return other?.username ?? 'Direct message'
  }
  return room.name?.trim() || (room.kind === 'project' ? 'Project chat' : 'Group chat')
}

function roomHint(room: ChatRoom): string {
  if (room.lastMessage?.body) return room.lastMessage.body
  if (room.lastMessage?.attachments.length) return 'Image'
  if (room.purpose) return room.purpose
  if (room.kind === 'project') return room.projectName ?? 'Project room'
  return room.visibility === 'discoverable' ? 'Open group' : 'Private group'
}

function roomIcon(room: ChatRoom) {
  if (room.kind === 'dm') return <MessageSquare size={15} />
  if (room.kind === 'project') return <Lock size={15} />
  return <Hash size={15} />
}

function statusLabel(status: ChatRequest['status']): string {
  if (status === 'approved') return 'Approved'
  if (status === 'rejected') return 'Rejected'
  return 'Pending review'
}

function AuthedImage({ attachment }: { attachment: ChatAttachment }) {
  const [url, setURL] = useState<string | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let active = true
    let objectURL: string | null = null
    api
      .fetchChatAttachment(attachment.id)
      .then((blob) => {
        if (!active) return
        objectURL = URL.createObjectURL(blob)
        setURL(objectURL)
      })
      .catch(() => {
        if (active) setFailed(true)
      })
    return () => {
      active = false
      if (objectURL) URL.revokeObjectURL(objectURL)
    }
  }, [attachment.id])

  if (failed) {
    return <div className="chat-image chat-image--failed">Image unavailable</div>
  }
  if (!url) {
    return <div className="chat-image chat-image--loading"><Spinner label="Loading image" /></div>
  }
  return (
    <a className="chat-image" href={url} target="_blank" rel="noreferrer" title={attachment.filename}>
      <img src={url} alt={attachment.filename} loading="lazy" />
    </a>
  )
}

function MessageRow({
  message,
  viewer,
  onDelete,
}: {
  message: ChatMessage
  viewer: User
  onDelete: (message: ChatMessage) => void
}) {
  const mine = message.senderId === viewer.id
  if (message.deletedAt) {
    return (
      <div className={cx('chat-message', mine && 'chat-message--mine')}>
        <div className="chat-message__bubble chat-message__bubble--deleted">Message removed</div>
      </div>
    )
  }
  return (
    <div className={cx('chat-message', mine && 'chat-message--mine')}>
      {!mine ? <Avatar name={message.senderName} size="sm" /> : null}
      <div className="chat-message__main">
        <div className="chat-message__meta">
          <strong>{mine ? 'You' : message.senderName}</strong>
          <span>{relativeDate(message.createdAt)}</span>
          {message.editedAt ? <span className="chat-message__edited">edited</span> : null}
          {mine ? (
            <button
              type="button"
              className="chat-message__delete"
              onClick={() => onDelete(message)}
              aria-label="Delete message"
              title="Delete message"
            >
              <Trash2 size={12} />
            </button>
          ) : null}
        </div>
        <div className="chat-message__bubble">
          {message.body ? <p>{message.body}</p> : null}
          {message.attachments.length > 0 ? (
            <div className="chat-message__images">
              {message.attachments.map((attachment) => (
                <AuthedImage key={attachment.id} attachment={attachment} />
              ))}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  )
}

function Composer({ roomId, onSent }: { roomId: number; onSent: () => void }) {
  const [body, setBody] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const fileInput = useRef<HTMLInputElement>(null)

  async function send() {
    if (busy) return
    if (!body.trim() && files.length === 0) return
    setBusy(true)
    setError(null)
    try {
      const uploaded = []
      for (const file of files) {
        uploaded.push(await api.uploadChatAttachment(roomId, file))
      }
      await api.sendChatMessage(roomId, body.trim(), uploaded.map((item) => item.id))
      setBody('')
      setFiles([])
      if (fileInput.current) fileInput.current.value = ''
      onSent()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not send the message.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="chat-composer">
      {files.length > 0 ? (
        <div className="chat-composer__files">
          {files.map((file, index) => (
            <span key={`${file.name}-${index}`} className="chat-composer__file">
              <ImagePlus size={13} />
              {file.name}
              <button
                type="button"
                onClick={() => setFiles((current) => current.filter((_, i) => i !== index))}
                aria-label={`Remove ${file.name}`}
              >
                <X size={12} />
              </button>
            </span>
          ))}
        </div>
      ) : null}

      <div className="chat-composer__row">
        <input
          ref={fileInput}
          type="file"
          accept="image/png,image/jpeg,image/gif,image/webp"
          multiple
          hidden
          onChange={(event) => {
            const picked = Array.from(event.target.files ?? [])
            setFiles((current) => [...current, ...picked].slice(0, 4))
          }}
        />
        <button
          type="button"
          className="icon-btn"
          onClick={() => fileInput.current?.click()}
          title="Attach an image"
          aria-label="Attach an image"
        >
          <ImagePlus size={17} />
        </button>
        <Textarea
          className="chat-composer__input"
          placeholder="Write a message…"
          rows={1}
          value={body}
          onChange={(event) => setBody(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && !event.shiftKey) {
              event.preventDefault()
              void send()
            }
          }}
        />
        <Button type="button" onClick={() => void send()} disabled={busy || (!body.trim() && files.length === 0)}>
          {busy ? <Spinner label="Sending" /> : <Send size={15} />}
          <span>Send</span>
        </Button>
      </div>
      {error ? <p className="field__error">{error}</p> : null}
    </div>
  )
}

function Conversation({ room }: { room: ChatRoom }) {
  const current = useSession((state) => state.user)
  const { data: messages = [], isLoading } = useChatMessages(room.id)
  const scroller = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const lastID = messages.at(-1)?.id

  useEffect(() => {
    const node = scroller.current
    if (node) node.scrollTop = node.scrollHeight
  }, [lastID])

  useEffect(() => {
    const newest = messages.at(-1)
    if (newest) void api.markChatRead(room.id, newest.id).catch(() => undefined)
  }, [room.id, lastID, messages])

  const title = current ? roomTitle(room, current.id) : room.name

  async function remove(message: ChatMessage) {
    if (!window.confirm('Delete this message for everyone?')) return
    try {
      await api.deleteChatMessage(message.id)
    } catch (err) {
      toast('Could not delete', err instanceof Error ? err.message : undefined, 'red')
    }
  }

  async function leave() {
    if (!window.confirm('Leave this chat? You can rejoin discoverable groups later.')) return
    try {
      await api.leaveChat(room.id)
      toast('Left chat', title, 'bone')
      navigate('/chat')
    } catch (err) {
      toast('Could not leave', err instanceof Error ? err.message : undefined, 'red')
    }
  }

  return (
    <section className="chat-room">
      <header className="chat-room__head">
        <div className="chat-room__title">
          <span className="chat-room__icon">{roomIcon(room)}</span>
          <div>
            <h1>{title}</h1>
            <p>
              {room.kind === 'project'
                ? `${room.projectName ?? 'Project'} · project members only`
                : room.kind === 'dm'
                  ? 'Private conversation'
                  : room.visibility === 'discoverable'
                    ? 'Open group · anyone can join'
                    : 'Private group'}
              {room.members?.length ? ` · ${room.members.length} members` : ''}
            </p>
          </div>
        </div>
        <div className="chat-room__actions">
          {room.kind !== 'dm' ? (
            <Button size="sm" variant="outline" onClick={() => void leave()}>
              <DoorOpen size={14} /> Leave
            </Button>
          ) : null}
        </div>
      </header>

      <div ref={scroller} className="chat-room__messages">
        {isLoading ? (
          <div className="loading-pane"><Spinner label="Loading messages" /></div>
        ) : messages.length === 0 ? (
          <EmptyState
            icon={<MessageSquare size={22} />}
            title="No messages yet"
            body="Say hello, share a thought, or drop an image to get the conversation moving."
          />
        ) : (
          current ? messages.map((message) => (
            <MessageRow key={message.id} message={message} viewer={current} onDelete={(item) => void remove(item)} />
          )) : null
        )}
      </div>

      <Composer
        roomId={room.id}
        onSent={() => {
          void queryClient.invalidateQueries({ queryKey: ['chat-messages', room.id] })
          void queryClient.invalidateQueries({ queryKey: ['chats'] })
        }}
      />
    </section>
  )
}

function DirectMessageDialog({
  open,
  onClose,
  onOpened,
}: {
  open: boolean
  onClose: () => void
  onOpened: (room: ChatRoom) => void
}) {
  const current = useSession((state) => state.user)
  const { data: users = [] } = useUsers()
  const [search, setSearch] = useState('')
  const [busy, setBusy] = useState<number | null>(null)

  const candidates = useMemo(() => {
    const needle = search.trim().toLowerCase()
    return users
      .filter((user) => user.id !== current?.id)
      .filter((user) => !needle || user.username.toLowerCase().includes(needle) || user.role.toLowerCase().includes(needle))
  }, [users, current?.id, search])

  async function openWith(user: User) {
    setBusy(user.id)
    try {
      const room = await api.openDM(user.id)
      onOpened(room)
      onClose()
    } catch (err) {
      toast('Could not open chat', err instanceof Error ? err.message : undefined, 'red')
    } finally {
      setBusy(null)
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title="Start a direct message" eyebrow="People">
      <Field label="Find someone" htmlFor="dm-search">
        <Input
          id="dm-search"
          placeholder="Search by name or role"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </Field>
      <div className="chat-picker">
        {candidates.length === 0 ? (
          <p className="meta-quiet">No one matches that search.</p>
        ) : (
          candidates.map((user) => (
            <button key={user.id} type="button" className="chat-picker__row" onClick={() => void openWith(user)}>
              <Avatar name={user.username} size="sm" />
              <span className="chat-picker__who">
                <strong>{user.username}</strong>
                <small>{user.role}</small>
              </span>
              {busy === user.id ? <Spinner label="Opening" /> : <MessageSquarePlus size={15} />}
            </button>
          ))
        )}
      </div>
    </Dialog>
  )
}

function JoinGroupDialog({
  open,
  onClose,
  onJoined,
}: {
  open: boolean
  onClose: () => void
  onJoined: (room: ChatRoom) => void
}) {
  const { data: rooms = [], isLoading } = useDiscoverChats(open)
  const { data: requests = [] } = useChatRequests()
  const [busy, setBusy] = useState<number | null>(null)

  async function join(room: ChatRoom) {
    setBusy(room.id)
    try {
      const joined = await api.joinChat(room.id)
      onJoined(joined)
      onClose()
    } catch (err) {
      toast('Could not join', err instanceof Error ? err.message : undefined, 'red')
    } finally {
      setBusy(null)
    }
  }

  return (
    <Dialog open={open} onClose={onClose} title="Discover groups" eyebrow="Open conversations" wide>
      <p className="form-note">Open groups are discoverable by default. Join one and keep the room lively.</p>
      {isLoading ? (
        <div className="loading-pane"><Spinner label="Finding groups" /></div>
      ) : rooms.length === 0 ? (
        <EmptyState title="Nothing open right now" body="Create a group and invite the team, or check back soon." />
      ) : (
        <div className="chat-picker">
          {rooms.map((room) => (
            <div key={room.id} className="chat-picker__row chat-picker__row--static">
              <span className="chat-picker__icon">{roomIcon(room)}</span>
              <span className="chat-picker__who">
                <strong>{room.name}</strong>
                <small>{room.purpose || `${room.members?.length ?? 0} members`}</small>
              </span>
              <Button size="sm" variant="outline" disabled={busy === room.id} onClick={() => void join(room)}>
                {busy === room.id ? <Spinner label="Joining" /> : <Plus size={13} />} Join
              </Button>
            </div>
          ))}
        </div>
      )}

      <div className="chat-requests">
        <span className="eyebrow">Your group requests</span>
        {requests.length === 0 ? (
          <p className="meta-quiet">No requests yet. New groups are approved immediately for now.</p>
        ) : (
          requests.map((request) => (
            <div key={request.id} className="chat-requests__row">
              <span>
                <strong>{request.name}</strong>
                <small>{request.purpose}</small>
              </span>
              <Badge tone={request.status === 'approved' ? 'green' : request.status === 'rejected' ? 'red' : 'gold'}>
                {statusLabel(request.status)}
              </Badge>
            </div>
          ))
        )}
      </div>
    </Dialog>
  )
}

function NewGroupDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: (room: ChatRoom) => void
}) {
  const current = useSession((state) => state.user)
  const { data: users = [] } = useUsers()
  const [name, setName] = useState('')
  const [purpose, setPurpose] = useState('')
  const [visibility, setVisibility] = useState<RoomVisibility>('discoverable')
  const [invites, setInvites] = useState<number[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const candidates = users.filter((user) => user.id !== current?.id)

  function toggle(id: number) {
    setInvites((current) => (current.includes(id) ? current.filter((value) => value !== id) : [...current, id]))
  }

  async function create() {
    setBusy(true)
    setError(null)
    try {
      const result = await api.createOpenRoom({ name, purpose, visibility, inviteIds: invites })
      toast('Group created', 'Approved immediately while the admin queue is being built.', 'gold')
      onCreated(result.room)
      onClose()
      setName('')
      setPurpose('')
      setInvites([])
      setVisibility('discoverable')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not create the group.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Create a group"
      eyebrow="New conversation"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button onClick={() => void create()} disabled={busy}>
            {busy ? <Spinner label="Creating" /> : <Plus size={15} />} Create group
          </Button>
        </>
      }
    >
      <Field label="Name" htmlFor="group-name" hint="3–48 characters">
        <Input id="group-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="Frontend guild" />
      </Field>
      <Field label="Purpose" htmlFor="group-purpose" hint="What is this room for?">
        <Textarea id="group-purpose" rows={3} value={purpose} onChange={(event) => setPurpose(event.target.value)} placeholder="Share patterns, reviews and frontend decisions." />
      </Field>
      <Field label="Visibility" htmlFor="group-visibility">
        <Select id="group-visibility" value={visibility} onChange={(event) => setVisibility(event.target.value as RoomVisibility)}>
          <option value="discoverable">Discoverable — anyone can find and join</option>
          <option value="private">Private — invited people only</option>
        </Select>
      </Field>
      <div className="chat-member-picker">
        <span className="field__label">Invite people</span>
        {candidates.map((user) => (
          <label key={user.id} className="chat-member-picker__row">
            <input type="checkbox" checked={invites.includes(user.id)} onChange={() => toggle(user.id)} />
            <Avatar name={user.username} size="sm" />
            <span>
              <strong>{user.username}</strong>
              <small>{user.role}</small>
            </span>
            {invites.includes(user.id) ? <Check size={14} /> : null}
          </label>
        ))}
      </div>
      {error ? <p className="field__error">{error}</p> : null}
    </Dialog>
  )
}

function RoomList({ rooms, selectedID }: { rooms: ChatRoom[]; selectedID: number }) {
  const current = useSession((state) => state.user)
  return (
    <div className="chat-list">
      {rooms.map((room) => (
        <Link
          key={room.id}
          to={`/chat/${room.id}`}
          className={cx('chat-list__row', room.id === selectedID && 'chat-list__row--active')}
        >
          <span className="chat-list__icon">{roomIcon(room)}</span>
          <span className="chat-list__main">
            <span className="chat-list__top">
              <strong>{current ? roomTitle(room, current.id) : room.name}</strong>
              {room.lastMessage ? <small>{relativeDate(room.lastMessage.createdAt)}</small> : null}
            </span>
            <span className="chat-list__hint">{roomHint(room)}</span>
          </span>
          {room.unreadCount > 0 ? <span className="chat-list__unread">{room.unreadCount}</span> : null}
        </Link>
      ))}
    </div>
  )
}

export function ChatPage() {
  const params = useParams()
  const navigate = useNavigate()
  const roomID = Number(params.roomId)
  const { data: rooms = [], isLoading } = useChats()
  const { data: room, isLoading: roomLoading } = useChat(roomID)
  const [dmOpen, setDmOpen] = useState(false)
  const [discoverOpen, setDiscoverOpen] = useState(false)
  const [groupOpen, setGroupOpen] = useState(false)
  const me = useMe().data

  function openRoom(next: ChatRoom) {
    void navigate(`/chat/${next.id}`)
  }

  if (!me && isLoading) {
    return <div className="loading-pane"><Spinner label="Opening chat" /><span>Loading conversations…</span></div>
  }

  return (
    <div className="chat-page">
      <aside className="chat-sidebar">
        <div className="chat-sidebar__head">
          <div>
            <span className="eyebrow">Chat</span>
            <h2>Conversations</h2>
          </div>
          <div className="chat-sidebar__tools">
            <button type="button" className="icon-btn" title="New direct message" aria-label="New direct message" onClick={() => setDmOpen(true)}>
              <UserPlus size={16} />
            </button>
            <button type="button" className="icon-btn" title="Discover groups" aria-label="Discover groups" onClick={() => setDiscoverOpen(true)}>
              <Search size={16} />
            </button>
            <button type="button" className="icon-btn" title="Create a group" aria-label="Create a group" onClick={() => setGroupOpen(true)}>
              <Plus size={16} />
            </button>
          </div>
        </div>

        {isLoading ? (
          <div className="loading-pane"><Spinner label="Loading conversations" /></div>
        ) : rooms.length === 0 ? (
          <div className="chat-sidebar__empty">
            <MessageSquare size={20} />
            <p>No conversations yet.</p>
            <Button size="sm" variant="outline" onClick={() => setDmOpen(true)}>
              <Users size={14} /> Start a DM
            </Button>
          </div>
        ) : (
          <RoomList rooms={rooms} selectedID={roomID} />
        )}
      </aside>

      <div className="chat-main">
        {roomID > 0 ? (
          roomLoading ? (
            <div className="loading-pane"><Spinner label="Opening room" /></div>
          ) : room ? (
            <Conversation room={room} />
          ) : (
            <EmptyState
              icon={<Lock size={22} />}
              title="Chat unavailable"
              body="You may not be a member of this room, or it has been archived."
              action={<Link to="/chat" className="btn btn--outline btn--sm">Back to chat</Link>}
            />
          )
        ) : (
          <div className="chat-welcome">
            <EmptyState
              icon={<MessageSquare size={26} />}
              title="Pick a conversation"
              body="Message a teammate privately, join an open group, or start a new room for the team."
              action={
                <div className="chat-welcome__actions">
                  <Button onClick={() => setDmOpen(true)}><UserPlus size={15} /> New direct message</Button>
                  <Button variant="outline" onClick={() => setDiscoverOpen(true)}><Search size={15} /> Discover groups</Button>
                  <Button variant="outline" onClick={() => setGroupOpen(true)}><Plus size={15} /> Create a group</Button>
                </div>
              }
            />
          </div>
        )}
      </div>

      <DirectMessageDialog open={dmOpen} onClose={() => setDmOpen(false)} onOpened={openRoom} />
      <JoinGroupDialog open={discoverOpen} onClose={() => setDiscoverOpen(false)} onJoined={openRoom} />
      <NewGroupDialog open={groupOpen} onClose={() => setGroupOpen(false)} onCreated={openRoom} />
    </div>
  )
}
